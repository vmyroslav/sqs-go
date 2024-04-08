package consumer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Handler is a generic interface for message handlers.
// The type parameter T specifies the type of message the handler accepts.
type Handler[T any] interface {
	Handle(ctx context.Context, msg T) error
}

type HandlerFunc[T any] func(ctx context.Context, msg T) error

func (f HandlerFunc[T]) Handle(ctx context.Context, msg T) error {
	return f(ctx, msg)
}

type Middleware[T any] func(next HandlerFunc[T]) HandlerFunc[T]

// poller is an interface for polling messages from SQS.
// The messages are transformed to the internal format and sent to the channel.
// The error channel is used to report errors that occurred during polling.
// The implementation should be able to handle context cancellation.
// poller should close the messages channel when it's done.
type poller interface {
	Poll(ctx context.Context, queueURL string, ch chan<- sqstypes.Message) error
}

type MessageAdapter[T any] interface {
	Transform(ctx context.Context, msg sqstypes.Message) (T, error)
}

type MessageAdapterFunc[T any] func(ctx context.Context, msg sqstypes.Message) (T, error)

func (f MessageAdapterFunc[T]) Transform(ctx context.Context, msg sqstypes.Message) (T, error) {
	return f(ctx, msg)
}

type acknowledger interface {
	Ack(ctx context.Context, msg sqstypes.Message) error
	Reject(ctx context.Context, msg sqstypes.Message) error
}

type Processor[T any] interface {
	Process(ctx context.Context, ch <-chan sqstypes.Message, handler Handler[T]) error
}

type SQSConsumer[T any] struct {
	cfg            Config
	poller         poller
	sqsClient      *sqs.Client
	messageAdapter MessageAdapter[T]
	confirmer      acknowledger
	processor      Processor[T]
	middlewares    []Middleware[T]

	stopSignalCh chan struct{}
	stoppedCh    chan struct{}
	isRunning    bool

	errors chan error

	logger *slog.Logger

	mu sync.RWMutex
}

func NewSQSConsumer[T any](
	cfg Config,
	sqsClient sqsConnector,
	messageAdapter MessageAdapter[T],
	middlewares []Middleware[T],
	logger *slog.Logger,
) *SQSConsumer[T] {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	c := &SQSConsumer[T]{
		cfg: cfg,
		poller: newSqsPoller(pollerConfig{
			MaxNumberOfMessages:  cfg.MaxNumberOfMessages,
			WaitTimeSeconds:      cfg.WaitTimeSeconds,
			VisibilityTimeout:    cfg.VisibilityTimeout,
			WorkerPoolSize:       cfg.PollerWorkerPoolSize,
			ErrorNumberThreshold: cfg.ErrorNumberThreshold,
		}, sqsClient, logger),
		processor: newProcessorSQS[T](
			processorConfig{WorkerPoolSize: cfg.ProcessorWorkerPoolSize},
			messageAdapter,
			newSyncAcknowledger(cfg.QueueURL, sqsClient),
			logger,
		),
		middlewares:  middlewares,
		stopSignalCh: make(chan struct{}, 1),
		stoppedCh:    make(chan struct{}, 1),
		logger:       logger,
	}

	return c
}

func (c *SQSConsumer[T]) Consume(ctx context.Context, queueURL string, messageHandler Handler[T]) error {
	var (
		// requires some tuning to find the optimal value depending on the message processing time and visibility timeout
		bufferSize = c.cfg.ProcessorWorkerPoolSize * 3

		msgs = make(chan sqstypes.Message, bufferSize) // poller should close this channel

		pollerErrCh  = make(chan error, 1)
		processErrCh = make(chan error, 1)

		handlerFunc = newMessageHandlerFunc(messageHandler)

		processCtx, cancelProcess = context.WithCancel(ctx)
		pollerCtx, cancelPoller   = context.WithCancel(ctx)
	)

	defer func() {
		cancelPoller()
		cancelProcess()

		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
	}()

	if c.IsRunning() {
		return fmt.Errorf("consumer is already running") // nolint:goerr113
	}

	c.mu.Lock()
	c.isRunning = true
	c.mu.Unlock()

	// apply middlewares
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handlerFunc = c.middlewares[i](handlerFunc)
	}

	go func() { pollerErrCh <- c.poller.Poll(pollerCtx, queueURL, msgs) }()
	go func() { processErrCh <- c.processor.Process(processCtx, msgs, handlerFunc) }()

	select {
	case <-ctx.Done():
		c.logger.InfoContext(ctx, "context is canceled. Shutting down consumer.")

		return ctx.Err()
	case <-c.stopSignalCh:
		c.logger.InfoContext(ctx, "stop signal received. Shutting down consumer.")
		cancelPoller()

		// Wait for the poller to finish
		if err := <-pollerErrCh; err != nil {
			c.logger.ErrorContext(ctx, "poller error", err)

			return err
		}

		// Wait for the processor to finish consuming the messages in the buffer
		if err := <-processErrCh; err != nil {
			c.logger.ErrorContext(ctx, "processing error", err)

			return err
		}

		close(c.stoppedCh)

		return nil
	case err := <-pollerErrCh:
		c.logger.Error("poller stopped unexpectedly", "error", err)

		return err
	case err := <-processErrCh:
		if err != nil {
			c.logger.ErrorContext(ctx, "processor stopped unexpectedly", "error", err)

			return err
		}
	}

	return nil
}

func (c *SQSConsumer[T]) Close() error {
	if !c.IsRunning() {
		return nil
	}

	c.logger.Debug("closing SQS consumer")

	c.stopSignalCh <- struct{}{}

	select {
	case <-c.stoppedCh:
		c.logger.Debug("SQS consumer stopped")

		return nil
	case <-time.After(time.Duration(c.cfg.GracefulShutdownTimeout) * time.Second):
		c.logger.Warn("SQS consumer did not stop in time")

		return fmt.Errorf("SQS consumer did not stop in time") // nolint:goerr113
	}
}

func (c *SQSConsumer[T]) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isRunning
}

func newMessageHandlerFunc[T any](handler Handler[T]) HandlerFunc[T] {
	return func(ctx context.Context, message T) error {
		return handler.Handle(ctx, message)
	}
}

//go:generate mockery --name=sqsConnector --filename=mock_sqs_connector.go --inpackage
type sqsConnector interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}
