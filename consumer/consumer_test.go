package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		handler = HandlerFunc[mockMessage](func(_ context.Context, msg mockMessage) error {
			mu.Lock()
			defer mu.Unlock()

			receivedMessages = append(receivedMessages, msg)

			return nil
		})
	)

	defer cancel()

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
		key := receivedMsg.Key

		_, ok := sqsClient.expectedMessages[key]
		if !ok {
			t.Fatalf("Received an unexpected message ID: %s", key)
		} else {
			delete(sqsClient.expectedMessages, key) // remove the message from the expected messages to check for duplicates
		}
	}
}

func TestSQSConsumer_Consume_MiddlewareOrder(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 1
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithCancel(context.Background())

		middlewareCalls []string
		mu              sync.RWMutex

		handler = HandlerFunc[mockMessage](func(_ context.Context, _ mockMessage) error {
			mu.Lock()
			defer mu.Unlock()

			middlewareCalls = append(middlewareCalls, "handler")

			return nil
		})

		middleware1 = func(next HandlerFunc[mockMessage]) HandlerFunc[mockMessage] {
			return func(ctx context.Context, msg mockMessage) error {
				mu.Lock()

				middlewareCalls = append(middlewareCalls, "middleware1")

				mu.Unlock()

				return next(ctx, msg)
			}
		}

		middleware2 = func(next HandlerFunc[mockMessage]) HandlerFunc[mockMessage] {
			return func(ctx context.Context, msg mockMessage) error {
				mu.Lock()

				middlewareCalls = append(middlewareCalls, "middleware2")

				mu.Unlock()

				return next(ctx, msg)
			}
		}

		middleware3 = func(next HandlerFunc[mockMessage]) HandlerFunc[mockMessage] {
			return func(ctx context.Context, msg mockMessage) error {
				mu.Lock()

				middlewareCalls = append(middlewareCalls, "middleware3")

				mu.Unlock()

				return next(ctx, msg)
			}
		}

		middleware4 = func(next HandlerFunc[mockMessage]) HandlerFunc[mockMessage] {
			return func(ctx context.Context, msg mockMessage) error {
				res := next(ctx, msg)

				mu.Lock()

				middlewareCalls = append(middlewareCalls, "middleware4")

				mu.Unlock()

				return res
			}
		}
	)

	defer cancel()

	c := NewSQSConsumer[mockMessage](
		sqsCfg,
		sqsClient,
		NewJSONMessageAdapter[mockMessage](),
		[]Middleware[mockMessage]{middleware1, middleware2, middleware3, middleware4},
		nil,
	)

	go func() {
		_ = c.Consume(ctx, sqsCfg.QueueURL, handler)
	}()

	assert.Eventually(t, func() bool {
		mu.RLock()
		defer mu.RUnlock()

		return len(middlewareCalls) == 5
	}, 500*time.Millisecond, 10*time.Millisecond)

	mu.RLock()

	finalCalls := make([]string, len(middlewareCalls))
	copy(finalCalls, middlewareCalls)
	mu.RUnlock()

	assert.Equal(t, []string{"middleware1", "middleware2", "middleware3", "handler", "middleware4"}, finalCalls)
}

func TestSQSConsumer_Consume_IsRunning(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		handler = HandlerFunc[mockMessage](func(_ context.Context, _ mockMessage) error {
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

	isRunningError := c.Consume(ctx, sqsCfg.QueueURL, handler)
	require.Error(t, isRunningError)
}

func TestSQSConsumer_Consume_ShouldListenToContextCancellation(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		handler = HandlerFunc[mockMessage](func(_ context.Context, _ mockMessage) error {
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

// TestSQSConsumer_Close_AllMessagesProcessed tests the graceful shutdown of the SQSConsumer.
// It verifies that all messages are processed before the consumer stops, even after the Close method is called.
// If the consumer does not process all messages within a predefined timeout, the test fails.
func TestSQSConsumer_Close_AllMessagesProcessed(t *testing.T) {
	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)
		processedMessages = make(chan sqstypes.Message, messagesToProduce)

		ctx, cancel = context.WithCancel(context.Background())

		handler = HandlerFunc[sqstypes.Message](func(_ context.Context, msg sqstypes.Message) error {
			time.Sleep(10 * time.Millisecond)

			processedMessages <- msg

			return nil
		})
	)

	defer cancel()

	c := NewSQSConsumer[sqstypes.Message](
		sqsCfg,
		sqsClient,
		NewJSONMessageAdapter[sqstypes.Message](),
		nil,
		nil,
	)

	go func() {
		err := c.Consume(ctx, "queueURL", handler)
		assert.NoError(t, err)
	}()

	// Allow the consumer to start and process some messages
	time.Sleep(20 * time.Millisecond)

	// Call Close and expect it to return within the stop timeout
	closeErr := c.Close()
	require.NoError(t, closeErr)

	// Check that all messages were processed
	assert.Len(t, processedMessages, len(sqsClient.expectedMessages))
}

func TestSQSConsumer_Close_NotRunningConsumer(t *testing.T) {
	t.Parallel()

	var (
		messagesToProduce = 100
		sqsClient         = newSQSConnectorMock(t, messagesToProduce)
	)

	c := NewSQSConsumer[sqstypes.Message](
		sqsCfg,
		sqsClient,
		NewJSONMessageAdapter[sqstypes.Message](),
		nil,
		nil,
	)

	err := c.Close()
	require.NoError(t, err)
}

func TestMessageAdapterFunc_Transform(t *testing.T) {
	t.Parallel()

	transformFunc := MessageAdapterFunc[string](func(_ context.Context, msg sqstypes.Message) (string, error) {
		return *msg.Body, nil
	})

	msg := sqstypes.Message{
		Body: aws.String("test message"),
	}

	result, err := transformFunc.Transform(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, "test message", result)
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
