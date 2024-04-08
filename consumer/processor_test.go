package consumer

import (
	"context"
	"log/slog"
	"os"
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
		handler   = HandlerFunc[sqstypes.Message](func(ctx context.Context, msg sqstypes.Message) error {
			return nil
		})
		callCh = make(chan struct{}, 1)
	)

	defer cancel()

	sqsClient.On("DeleteMessage", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
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
		handler   = HandlerFunc[sqstypes.Message](func(ctx context.Context, msg sqstypes.Message) error {
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
		queueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue"
		msgs     = make(chan sqstypes.Message, 1)
		pCfg     = processorConfig{
			WorkerPoolSize: 2,
		}
		sqsClient = newMockSqsConnector(t)
		logger    = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		msgBody   = "original message"
	)

	sqsClient.On(
		"DeleteMessage",
		mock.Anything,
		mock.Anything,
	).Return(nil, nil)

	p := newProcessorSQS[sqstypes.Message](
		pCfg,
		NewDummyAdapter[sqstypes.Message](),
		newSyncAcknowledger(queueURL, sqsClient),
		logger,
	)

	msgs <- sqstypes.Message{Body: aws.String(msgBody)}
	defer close(msgs)

	handlerCalled := make(chan bool, 1)

	handler := HandlerFunc[sqstypes.Message](func(ctx context.Context, msg sqstypes.Message) error {
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
