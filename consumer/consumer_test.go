package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
)

var (
	_      sqsConnector = (*mockSQSConnector)(nil)
	sqsCfg              = Config{
		QueueURL:                "https://sqs.eu-west-1.amazonaws.com/123456789012/queue-name",
		ProcessorWorkerPoolSize: 10,
		PollerWorkerPoolSize:    2,
		MaxNumberOfMessages:     10,
		WaitTimeSeconds:         1,
		VisibilityTimeout:       1,
		ErrorNumberThreshold:    -1,
		GracefulShutdownTimeout: 3,
		ReturnErrors:            false,
	}
)

type mockMessage struct {
	Key     string
	Payload string
}

// TestSQSConsumer_Consume_SuccessfullyProcessMessages tests the successful consumption of messages from an SQS queue.
// It creates a mock SQS client that produces a predefined number of messages and a consumer that consumes these messages.
// The test verifies that all produced messages are successfully consumed and processed by the consumer.
// If the consumer does not consume all messages within a predefined timeout, the test fails.
func TestSQSConsumer_Consume_SuccessfullyProcessMessages(t *testing.T) {
	var (
		messagesToProduce = 1000
		receivedMessages  = make([]mockMessage, 0, messagesToProduce)
		mu                sync.RWMutex
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithCancel(context.Background())

		handler = HandlerFunc[mockMessage](func(ctx context.Context, msg mockMessage) error {
			mu.Lock()
			defer mu.Unlock()
			receivedMessages = append(receivedMessages, msg)

			return nil
		})
	)

	defer func() {
		cancel()
	}()

	c := NewSQSConsumer[mockMessage](sqsCfg, sqsClient, NewJSONMessageAdapter[mockMessage](), nil, nil)

	go func() {
		_ = c.Consume(ctx, "queueURL", handler)
	}()

	assert.Eventuallyf(t, func() bool {
		mu.RLock()
		defer mu.RUnlock()

		return len(receivedMessages) == messagesToProduce
	}, 1*time.Second, 10*time.Millisecond, "Did not receive the expected number of messages")

	// Check that all the expected messages have been received
	for _, receivedMsg := range receivedMessages {
		key := fmt.Sprintf("%v", receivedMsg.Key)

		_, ok := sqsClient.expectedMessages[key]
		if !ok {
			t.Fatalf("Received an unexpected message ID: %s", key)
		} else {
			delete(sqsClient.expectedMessages, key) // remove the message from the expected messages to check for duplicates
		}
	}
}

func TestSQSConsumer_Consume_IsRunning(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		handler = HandlerFunc[mockMessage](func(ctx context.Context, msg mockMessage) error {
			return nil
		})
	)

	defer cancel()

	c := NewSQSConsumer[mockMessage](
		sqsCfg,
		sqsClient,
		NewJSONMessageAdapter[mockMessage](),
		nil,
		nil,
	)

	go func() {
		_ = c.Consume(ctx, sqsCfg.QueueURL, handler)
	}()

	assert.Eventually(t, func() bool {
		return c.IsRunning()
	}, 500*time.Millisecond, 10*time.Millisecond)
}
func TestSQSConsumer_Consume_ShouldListenToContextCancellation(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		handler = HandlerFunc[mockMessage](func(ctx context.Context, msg mockMessage) error {
			return nil
		})
		errCh = make(chan error, 1)
	)

	defer cancel()

	c := NewSQSConsumer[mockMessage](
		sqsCfg,
		sqsClient,
		NewJSONMessageAdapter[mockMessage](),
		nil,
		nil,
	)

	go func() {
		errCh <- c.Consume(ctx, sqsCfg.QueueURL, handler)
	}()

	assert.Eventually(t, func() bool {
		return !c.IsRunning()
	}, 1*time.Second, 10*time.Millisecond)

	assert.ErrorIs(t, <-errCh, context.DeadlineExceeded)
}

type mockSQSConnector struct {
	expectedMessages    map[string]sqstypes.Message
	messageChan         chan sqstypes.Message
	maxNumberOfMessages int
	numberToProduce     int
	err                 error

	mu sync.RWMutex
}

// newSQSConnectorMock creates a mock SQS connector that produces a number of messages and simulates the messages coming from SQS.
func newSQSConnectorMock(t *testing.T, numMsgs int) *mockSQSConnector {
	t.Helper()
	// Prepare the expected messages and the channel to simulate the messages coming from SQS
	messageChan := make(chan sqstypes.Message, numMsgs)
	expectedMessages := make(map[string]sqstypes.Message)

	for i := 0; i < numMsgs; i++ {
		m := mockMessage{
			Key:     fmt.Sprintf("message-key-%d", i),
			Payload: fmt.Sprintf("message-payload-%d", i),
		}

		mbytes, err := json.Marshal(m)
		require.NoError(t, err)

		messageChan <- sqstypes.Message{
			MessageAttributes: map[string]sqstypes.MessageAttributeValue{
				"key": {
					DataType:    aws.String("String"),
					StringValue: aws.String(m.Key),
				},
			},
			Body: aws.String(string(mbytes)),
		}
	}

	return &mockSQSConnector{
		expectedMessages:    expectedMessages,
		messageChan:         messageChan,
		numberToProduce:     numMsgs,
		maxNumberOfMessages: 10,
		mu:                  sync.RWMutex{},
	}
}

func (m *mockSQSConnector) ReceiveMessage(_ context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	messages := make([]sqstypes.Message, 0)
	numMessagesToRead := rand.Intn(m.maxNumberOfMessages) + 1

	for i := 0; i < numMessagesToRead; i++ {
		select {
		case msg := <-m.messageChan:
			messages = append(messages, msg)
			m.expectedMessages[*msg.MessageAttributes["key"].StringValue] = msg
		default: // No more messages to read
		}
	}

	return &sqs.ReceiveMessageOutput{
		Messages: messages,
	}, nil
}

func (m *mockSQSConnector) DeleteMessage(_ context.Context, _ *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	return nil, nil // nolint: nilnil
}
