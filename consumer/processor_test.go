package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_Process_WhenHandlerWorkerPoolSizeIsZero_ReturnsError(t *testing.T) {
	t.Parallel()

	p := &processorSQS[sqstypes.Message]{
		cfg: processorConfig{
			WorkerPoolSize: 0,
		},
	}
	err := p.Process(context.Background(), make(chan sqstypes.Message), nil)
	assert.Error(t, err)
}

func Test_Process_WhenMessagesChannelIsClosed(t *testing.T) {
	t.Parallel()

	var (
		queueURL    = "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue"
		logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		msgs        = make(chan sqstypes.Message, 1)
		errCh       = make(chan error, 1)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
		pCfg        = processorConfig{
			WorkerPoolSize: 2,
		}
		sqsClient = newMockSqsConnector(t)
		handler   = HandlerFunc[sqstypes.Message](func(_ context.Context, _ sqstypes.Message) error {
			return nil
		})
		callCh = make(chan struct{}, 1)
	)

	defer cancel()

	sqsClient.On("DeleteMessage", mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) {
			callCh <- struct{}{} // mock the processing of the message
		}).Return(&sqs.DeleteMessageOutput{}, nil)

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		newSyncAcknowledger(queueURL, sqsClient),
		logger,
	)

	go func() {
		errCh <- p.Process(ctx, msgs, handler)
	}()

	msgs <- sqstypes.Message{}

	<-callCh // wait to process the message
	close(msgs)

	assert.NoError(t, <-errCh)
}

func Test_Process_WhenContextIsCancelled_ExitsWithoutError(t *testing.T) {
	t.Parallel()

	var (
		queueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue"
		errCh    = make(chan error, 1)
		pCfg     = processorConfig{
			WorkerPoolSize: 2,
		}
		sqsClient = newMockSqsConnector(t)
		logger    = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		handler   = HandlerFunc[sqstypes.Message](func(_ context.Context, _ sqstypes.Message) error {
			return nil
		})
	)

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		newSyncAcknowledger(queueURL, sqsClient),
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	go func() {
		errCh <- p.Process(ctx, make(chan sqstypes.Message), handler)
	}()

	err := <-errCh
	assert.NoError(t, err)
}

func Test_Process_WhenMessageIsReceived_CallsHandlerWithCorrectMessage(t *testing.T) {
	t.Parallel()

	var (
		msgs = make(chan sqstypes.Message, 1)
		pCfg = processorConfig{
			WorkerPoolSize: 2,
		}
		mockAck = newMockAcknowledger(nil, 1)
		logger  = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		msgBody = "original message"
	)

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		mockAck,
		logger,
	)

	msgs <- sqstypes.Message{Body: aws.String(msgBody)}
	defer close(msgs)

	handlerCalled := make(chan bool, 1)

	handler := HandlerFunc[sqstypes.Message](func(_ context.Context, msg sqstypes.Message) error {
		handlerCalled <- true

		assert.Equal(t, msgBody, *msg.Body)

		return nil
	})

	go func() {
		err := p.Process(context.Background(), msgs, handler)
		assert.NoError(t, err)
	}()

	select {
	case hc := <-handlerCalled:
		assert.True(t, hc)
	case <-time.After(1 * time.Second):
		t.Error("Test did not complete within the expected time")
	}
}

func Test_Process_HandlingAckErrors(t *testing.T) {
	t.Parallel()

	var (
		msgs = make(chan sqstypes.Message, 1)
		pCfg = processorConfig{
			WorkerPoolSize: 2,
		}
		logger  = slog.New(slog.DiscardHandler)
		msgBody = "original message"

		handler = HandlerFunc[sqstypes.Message](func(_ context.Context, _ sqstypes.Message) error {
			return nil
		})
	)

	defer close(msgs)

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		newMockAcknowledger(fmt.Errorf("failed to ack message"), 3),
		logger,
	)

	go func() {
		err := p.Process(context.Background(), msgs, handler)
		assert.NoError(t, err)
	}()

	msgs <- sqstypes.Message{Body: aws.String(msgBody)}
}

// mockAcknowledger is a mock implementation of acknowledger interface
type mockAcknowledger struct {
	err               error
	numberOfSeqErrors int
	callsCount        int

	mu sync.RWMutex
}

func newMockAcknowledger(err error, numberOfSeqErrors int) *mockAcknowledger {
	return &mockAcknowledger{
		err:               err,
		numberOfSeqErrors: numberOfSeqErrors,
		mu:                sync.RWMutex{},
	}
}

func (m *mockAcknowledger) Ack(_ context.Context, _ sqstypes.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callsCount++

	if m.callsCount < m.numberOfSeqErrors {
		return m.err
	}

	return nil
}

func (m *mockAcknowledger) Reject(_ context.Context, _ sqstypes.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callsCount++

	if m.callsCount < m.numberOfSeqErrors {
		return m.err
	}

	return nil
}

func Test_Process_WhenHandlerReturnsError_MessageRejected(t *testing.T) {
	t.Parallel()

	var (
		msgs = make(chan sqstypes.Message, 1)
		pCfg = processorConfig{
			WorkerPoolSize: 1,
		}
		logger       = slog.New(slog.DiscardHandler)
		msgBody      = "test message"
		rejectCalled = make(chan bool, 1)
	)

	mockAck := &trackingAcknowledger{
		rejectCalled: rejectCalled,
	}

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		mockAck,
		logger,
	)

	handler := HandlerFunc[sqstypes.Message](func(_ context.Context, _ sqstypes.Message) error {
		return fmt.Errorf("handler error")
	})

	go func() {
		err := p.Process(context.Background(), msgs, handler)
		assert.NoError(t, err)
	}()

	msgs <- sqstypes.Message{Body: aws.String(msgBody)}
	defer close(msgs)

	select {
	case called := <-rejectCalled:
		assert.True(t, called)
	case <-time.After(1 * time.Second):
		t.Error("Reject was not called within expected time")
	}
}

type trackingAcknowledger struct {
	rejectCalled chan bool
}

func (t *trackingAcknowledger) Ack(_ context.Context, _ sqstypes.Message) error {
	return nil
}

func (t *trackingAcknowledger) Reject(_ context.Context, _ sqstypes.Message) error {
	t.rejectCalled <- true
	return nil
}
