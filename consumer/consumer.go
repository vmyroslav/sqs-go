package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer/observability"
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

type SQSConsumer[T any] struct { // nolint:govet
	cfg         Config
	poller      poller
	processor   Processor[T]
	middlewares []Middleware[T]

	stopSignalCh chan struct{}
	stoppedCh    chan struct{}
	isRunning    bool
	isClosing    bool

	logger  *slog.Logger
	tracer  observability.SQSTracer
	metrics observability.SQSMetrics

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
		logger = slog.New(slog.DiscardHandler)
	}

	if cfg.Observability == nil {
		cfg.Observability = observability.NewConfig() // disabled by default
	}

	var (
		tracer  = observability.NewTracer(cfg.Observability)
		metrics = observability.NewMetrics(cfg.Observability)

		obsMessageAdapter = newObservableMessageAdapter[T](
			messageAdapter, tracer, metrics, cfg.QueueURL,
		)

		obsAcknowledger = newObservableAcknowledger(
			newSyncAcknowledger(cfg.QueueURL, sqsClient), tracer, metrics, cfg.QueueURL,
		)
	)

	obsMiddleware := observabilityMiddleware[T](tracer, metrics, cfg.QueueURL)

	// inject observability middleware as FIRST middleware
	allMiddlewares := []Middleware[T]{obsMiddleware}
	allMiddlewares = append(allMiddlewares, middlewares...)

	c := &SQSConsumer[T]{
		cfg: cfg,
		poller: newSqsPoller(pollerConfig{
			MaxNumberOfMessages:  cfg.MaxNumberOfMessages,
			WaitTimeSeconds:      cfg.WaitTimeSeconds,
			VisibilityTimeout:    cfg.VisibilityTimeout,
			WorkerPoolSize:       cfg.PollerWorkerPoolSize,
			ErrorNumberThreshold: cfg.ErrorNumberThreshold,
		}, sqsClient, logger, tracer, metrics),
		processor: newProcessorSQS[T](
			processorConfig{
				WorkerPoolSize: cfg.ProcessorWorkerPoolSize,
				QueueURL:       cfg.QueueURL,
			},
			obsMessageAdapter,
			obsAcknowledger,
			logger,
			tracer,
			cfg.Observability.Propagator(),
		),
		middlewares:  allMiddlewares,
		stopSignalCh: make(chan struct{}, 1),
		stoppedCh:    make(chan struct{}, 1),
		logger:       logger,
		tracer:       tracer,
		metrics:      metrics,
	}

	return c
}

// Consume starts consuming messages from the specified SQS queue using the provided message handler.
// The method respects context cancellation but for graceful shutdown with proper message processing
// completion, use the Close() method instead of canceling the context.
//
// Context behavior:
//   - Context cancellation will immediately stop the consumer and return ctx.Err()
//   - For graceful shutdown, call Close() which allows in-flight messages to complete processing
//
// Returns an error if the consumer fails to start or encounters an unrecoverable error.
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
		c.isClosing = false
		c.mu.Unlock()

		// record final metrics
		c.metrics.Gauge(ctx, observability.MetricActiveWorkers, 0,
			observability.WithQueueURLMetric(queueURL),
			observability.WithProcessType("poller"),
		)
		c.metrics.Gauge(ctx, observability.MetricActiveWorkers, 0,
			observability.WithQueueURLMetric(queueURL),
			observability.WithProcessType("processor"),
		)
	}()

	c.mu.Lock()

	if c.isRunning {
		c.mu.Unlock()

		return fmt.Errorf("consumer is already running")
	}

	c.isRunning = true
	c.mu.Unlock()

	// record initial metrics
	c.metrics.Gauge(ctx, observability.MetricActiveWorkers, int64(c.cfg.PollerWorkerPoolSize),
		observability.WithQueueURLMetric(queueURL),
		observability.WithProcessType("poller"),
	)
	c.metrics.Gauge(ctx, observability.MetricActiveWorkers, int64(c.cfg.ProcessorWorkerPoolSize),
		observability.WithQueueURLMetric(queueURL),
		observability.WithProcessType("processor"),
	)
	c.metrics.Gauge(ctx, observability.MetricBufferSize, int64(bufferSize),
		observability.WithQueueURLMetric(queueURL),
	)

	// apply middlewares
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handlerFunc = c.middlewares[i](handlerFunc)
	}

	go func() { pollerErrCh <- c.poller.Poll(pollerCtx, queueURL, msgs) }()
	go func() { processErrCh <- c.processor.Process(processCtx, msgs, handlerFunc) }()

	select {
	case <-ctx.Done():
		c.logger.InfoContext(ctx, "context is canceled. Shutting down consumer.")

		return ctx.Err() // nolint:wrapcheck
	case <-c.stopSignalCh:
		c.logger.InfoContext(ctx, "stop signal received. Shutting down consumer.")
		cancelPoller()

		// wait for the poller to finish
		if err := <-pollerErrCh; err != nil {
			c.logger.ErrorContext(ctx, "poller error", "error", err)

			return err
		}

		// wait for the processor to finish consuming the messages in the buffer
		if err := <-processErrCh; err != nil {
			c.logger.ErrorContext(ctx, "processing error", "error", err)

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

// Close performs a graceful shutdown of the SQS consumer.
// This is the recommended way to stop the consumer as it ensures proper cleanup:
//
// Shutdown sequence:
//  1. Stops polling for new messages from SQS
//  2. Allows in-flight messages in the buffer to complete processing
//  3. Waits for all worker goroutines to finish cleanly
//  4. Respects the configured GracefulShutdownTimeout
//
// Returns an error if the consumer fails to stop within the configured timeout period.
func (c *SQSConsumer[T]) Close() error {
	c.mu.Lock()

	if !c.isRunning || c.isClosing {
		c.mu.Unlock()
		return nil
	}

	// announce our intent to close
	c.isClosing = true

	c.logger.Debug("closing SQS consumer")

	c.stopSignalCh <- struct{}{}

	c.mu.Unlock()

	select {
	case <-c.stoppedCh:
		c.logger.Debug("SQS consumer stopped")

		return nil
	case <-time.After(time.Duration(c.cfg.GracefulShutdownTimeout) * time.Second):
		c.logger.Warn("SQS consumer did not stop in time")

		return fmt.Errorf("SQS consumer did not stop in time")
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
