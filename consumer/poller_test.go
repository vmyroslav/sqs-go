package consumer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSqsPoller_Poll(t *testing.T) { // nolint: gocognit
	var (
		cfg = pollerConfig{
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     1,
			VisibilityTimeout:   30,
			WorkerPoolSize:      2,
		}

		queueURL = "testQueueURL"
	)

	// Test case 1: Successful polling of messages
	t.Run("successful polling", func(t *testing.T) {
		var (
			sqsClient   = newMockSqsConnector(t)
			ctx, cancel = context.WithCancel(context.Background())
			messagesCh  = make(chan sqstypes.Message, 100)
			logger      = slog.New(slog.NewTextHandler(io.Discard, nil))
			p           poller
		)

		// Prepare the expected messages and the channel to simulate the messages coming from SQS
		// Produce 100 messages to be read by the poller
		messageChan := make(chan sqstypes.Message, 100)
		expectedMessages := make(map[string]sqstypes.Message)

		defer func() {
			close(messageChan)
			cancel()
		}()

		for i := 0; i < 100; i++ {
			messageID := fmt.Sprintf("testMessageID%d", i)
			messageBody := fmt.Sprintf("testMessageBody%d", i)
			msg := sqstypes.Message{
				MessageId: aws.String(messageID),
				Body:      aws.String(messageBody),
			}
			expectedMessages[messageID] = msg
			messageChan <- msg
		}

		// Mock the ReceiveMessage method to return the messages from the channel
		// The number of messages to be read is random, up to the maximum number of messages
		mockMessagesOutputFunc := func() *sqs.ReceiveMessageOutput {
			messages := make([]sqstypes.Message, 0)
			numMessagesToRead := rand.Intn(int(cfg.MaxNumberOfMessages)) + 1

			for i := 0; i < numMessagesToRead; i++ {
				select {
				case msg := <-messageChan:
					messages = append(messages, msg)
				default:
				}
			}

			return &sqs.ReceiveMessageOutput{
				Messages: messages,
			}
		}

		sqsClient.On(
			"ReceiveMessage",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.Anything,
			mock.Anything,
		).Return(mockMessagesOutputFunc(), nil)

		p = newSqsPoller(cfg, sqsClient, logger)

		go func() {
			err := p.Poll(ctx, queueURL, messagesCh)
			assert.NoError(t, err)
		}()

		var (
			receivedMessages []sqstypes.Message
			timeout          = time.After(1 * time.Second)
		)

	loop:
		for {
			select {
			case msg := <-messagesCh:
				receivedMessages = append(receivedMessages, msg)
				if len(receivedMessages) == 100 {
					break loop
				}
			case <-timeout:
				break loop // Timeout to prevent indefinite waiting if messages are missing
			}
		}

		assert.Equal(t, len(expectedMessages), len(receivedMessages), "Did not receive the expected number of messages")

		for _, receivedMsg := range receivedMessages {
			expectedMsg, ok := expectedMessages[*receivedMsg.MessageId]
			if !ok {
				t.Errorf("Received an unexpected message ID: %s", *receivedMsg.MessageId)
			} else {
				assert.Equal(t, expectedMsg.Body, receivedMsg.Body, "Message body does not match for ID: "+*receivedMsg.MessageId)
			}
		}
	})

	// Test case 2: Proper finishing of the processor when the context is done
	t.Run("context done", func(t *testing.T) {
		var (
			sqsClient   = newMockSqsConnector(t)
			ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			logger      = slog.New(slog.NewTextHandler(io.Discard, nil))
		)

		defer cancel()

		sqsClient.On(
			"ReceiveMessage",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.Anything,
			mock.Anything,
		).Run(func(_ mock.Arguments) {
			<-time.After(5 * time.Millisecond) // Simulate a delay in the ReceiveMessage method
		}).Return(&sqs.ReceiveMessageOutput{
			Messages: []sqstypes.Message{
				{
					MessageId: aws.String("testMessageID"),
					Body:      aws.String("testMessageBody"),
				},
			},
		}, nil)

		var (
			p     = newSqsPoller(cfg, sqsClient, logger)
			ch    = make(chan sqstypes.Message, 100)
			errCh = make(chan error, 1)
		)

		go func() {
			errCh <- p.Poll(ctx, queueURL, ch)
		}()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Error("Test did not complete within the expected time")
		}

		// Drain the channel
		for len(ch) > 0 {
			<-ch
		}

		select {
		case _, ok := <-ch:
			assert.False(t, ok, "channel should be closed when context is done")
		case <-time.After(2 * time.Second):
			t.Error("Test did not complete within the expected time")
		}
	})

	// Test case 3: Errors exceeding the threshold during polling
	t.Run("errors exceeding the threshold during polling", func(t *testing.T) {
		var (
			ctx, cancel = context.WithCancel(context.Background())
			messagesCh  = make(chan sqstypes.Message, 100)
			errCh       = make(chan error, 1)
			sqsClient   = newMockSqsConnector(t)
			logger      = slog.New(slog.NewTextHandler(io.Discard, nil))
		)

		cfg.ErrorNumberThreshold = 1
		p := newSqsPoller(cfg, sqsClient, logger)

		defer cancel()

		sqsClient.On(
			"ReceiveMessage",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.Anything,
			mock.Anything,
		).Return(nil, errors.New("an error occurred"))

		go func() {
			errCh <- p.Poll(ctx, queueURL, messagesCh)
		}()

		assert.Error(t, <-errCh)
	})

	// Test case 4: Errors occurring but not exceeding the threshold
	t.Run("error occurs but does not exceed the threshold", func(t *testing.T) {
		var (
			ctx, cancel = context.WithCancel(context.Background())
			messagesCh  = make(chan sqstypes.Message, 100)
			errCh       = make(chan error, 1)
			errorCount  int32
			sqsClient   = newMockSqsConnector(t)
			logger      = slog.New(slog.NewTextHandler(io.Discard, nil))
		)

		cfg.ErrorNumberThreshold = 5
		p := newSqsPoller(cfg, sqsClient, logger)

		defer cancel()

		sqsClient.On(
			"ReceiveMessage",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.Anything,
			mock.Anything,
		).Return(
			func(_ context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
				if atomic.LoadInt32(&errorCount) < 3 {
					atomic.AddInt32(&errorCount, 1)

					return nil, errors.New("an error occurred")
				}

				// return valid messages for the subsequent calls
				return &sqs.ReceiveMessageOutput{
					Messages: []sqstypes.Message{
						{
							MessageId: aws.String("testMessageID"),
							Body:      aws.String("testMessageBody"),
						},
					},
				}, nil
			},
		)

		go func() {
			errCh <- p.Poll(ctx, queueURL, messagesCh)
		}()

		select {
		case msg := <-messagesCh:
			assert.Equal(t, "testMessageID", *msg.MessageId)
		case err := <-errCh:
			t.Errorf("unexpected error: %v", err)
		case <-time.After(2 * time.Second):
			t.Error("Test did not complete within the expected time")
		}
	})
}
