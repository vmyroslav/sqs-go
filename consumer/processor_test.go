package consumer

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"log/slog"
	"os"
	"testing"
	"time"

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
			callCh <- struct{}{}
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
	println(1)

	<-callCh
	println(2)

	close(msgs)
	println(3)
	err := <-errCh
	println(4)
	assert.NoError(t, err)
}

//func Test_Process_WhenContextIsCancelled_ExitsWithoutError(t *testing.T) {
//	t.Parallel()
//
//	var (
//		queueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue"
//		errCh    = make(chan error, 1)
//	)
//
//	p := newProcessorSQS(processorConfig{
//		processorWorkerPoolSize: 2,
//		queueURL:                queueURL,
//	}, &ProtoSQSMessageAdapter{}, newSyncAcknowledger(nil), ctxd.NoOpLogger{})
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
//	defer cancel()
//
//	go func() {
//		errCh <- p.Process(ctx, make(chan sqstypes.Message), nil)
//	}()
//
//	err := <-errCh
//	assert.NoError(t, err)
//}
//
//func Test_Process_WhenMessageIsReceived_CallsMessageHandlerWithCorrectMessage(t *testing.T) {
//	t.Parallel()
//
//	var (
//		sqsClient = newMockSqsConnector(t)
//		msgs      = make(chan sqstypes.Message, 1)
//	)
//
//	sqsClient.On("DeleteMessage", mock.Anything, mock.Anything).Return(nil, nil)
//
//	p := newProcessorSQS(processorConfig{
//		processorWorkerPoolSize: 1,
//	}, &mockMessageAdapter{
//		TransformFunc: func(ctx context.Context, msg sqstypes.Message) (Message, error) {
//			return Message{Payload: "transformed message"}, nil
//		},
//	}, newSyncAcknowledger(sqsClient), ctxd.NoOpLogger{})
//
//	msgs <- sqstypes.Message{Body: aws.String("original message")}
//	close(msgs)
//
//	handlerCalled := false
//
//	handler := MessageHandleFunc(func(ctx context.Context, msg Message) error {
//		handlerCalled = true
//		assert.Equal(t, "transformed message", msg.Payload)
//
//		return nil
//	})
//	err := p.Process(context.Background(), msgs, handler)
//	assert.NoError(t, err)
//	assert.True(t, handlerCalled)
//}

type mockMessageAdapter struct {
	TransformFunc func(ctx context.Context, msg sqstypes.Message) (Message, error)
}

func (m *mockMessageAdapter) Transform(ctx context.Context, msg sqstypes.Message) (Message, error) {
	return m.TransformFunc(ctx, msg)
}
