package consumer

import (
	"context"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Message interface{}

// Handler is a generic interface for message handlers. The type parameter T
// specifies the type of message the handler accepts.
type Handler[T Message] interface {
	Handle(ctx context.Context, msg T) error
}

type HandlerFunc[T Message] func(ctx context.Context, msg T) error

func (f HandlerFunc[T]) Handle(ctx context.Context, msg T) error {
	return f(ctx, msg)
}

type Middleware[T Message] func(next HandlerFunc[T]) HandlerFunc[T]

// Poller is an interface for polling messages from SQS.
// The messages are transformed to the internal format and sent to the channel.
// The error channel is used to report errors that occurred during polling.
// The implementation should be able to handle context cancellation.
type Poller interface {
	Poll(ctx context.Context, queueURL string, ch chan<- sqstypes.Message) error
}

type MessageAdapter[T Message] interface {
	Transform(ctx context.Context, msg sqstypes.Message) (T, error)
}

type MessageAdapterFunc[T Message] func(ctx context.Context, msg sqstypes.Message) (T, error)

func (f MessageAdapterFunc[T]) Transform(ctx context.Context, msg sqstypes.Message) (T, error) {
	return f(ctx, msg)
}

type Confirmer interface {
	Ack(ctx context.Context, msg sqstypes.Message) error
	Reject(ctx context.Context, msg sqstypes.Message) error
}

type Processor[T Message] interface {
	Process(ctx context.Context, ch <-chan sqstypes.Message, handler Handler[T]) error
}

type ConsumerSQS[T Message] struct {
	cfg            Config
	poller         Poller
	sqsClient      *sqs.Client
	messageAdapter MessageAdapter[T]
	confirmer      Confirmer
	processor      Processor[T]
	middlewares    []Middleware[T]

	stopSignalCh chan struct{}
	stoppedCh    chan struct{}

	logger *slog.Logger
}

func NewConsumerSQS[T Message](
	cfg Config,
	poller Poller,
	sqsClient *sqs.Client,
	messageAdapter MessageAdapter[T],
	processor Processor[T],
	middlewares []Middleware[T],
	logger *slog.Logger,
) *ConsumerSQS[T] {
	c := &ConsumerSQS[T]{
		cfg:            cfg,
		poller:         poller,
		sqsClient:      sqsClient,
		messageAdapter: messageAdapter,
		confirmer:      newSyncAcknowledger(sqsClient, cfg.QueueURL),
		processor:      processor,
		middlewares:    middlewares,
		stopSignalCh:   make(chan struct{}, 1),
		stoppedCh:      make(chan struct{}, 1),
		logger:         logger,
	}

	if c.logger == nil {
		c.logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	return c
}

func NewDefaultConsumer[T Message](
	cfg Config,
	sqsClient *sqs.Client,
	messageAdapter MessageAdapter[T],
	middlewares []Middleware[T],
	logger *slog.Logger,
) *ConsumerSQS[T] {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	c := &ConsumerSQS[T]{
		cfg: cfg,
		poller: newSqsPoller(pollerConfig{
			MaxNumberOfMessages:  cfg.MaxNumberOfMessages,
			WaitTimeSeconds:      cfg.WaitTimeSeconds,
			VisibilityTimeout:    cfg.VisibilityTimeout,
			WorkerPoolSize:       cfg.PollerWorkerPoolSize,
			ErrorNumberThreshold: cfg.ErrorNumberThreshold,
		}, sqsClient, logger),
		processor: newProcessorSQS[T](
			processorConfig{HandlerWorkerPoolSize: cfg.HandlerWorkerPoolSize},
			messageAdapter,
			newSyncAcknowledger(sqsClient, cfg.QueueURL),
			middlewares,
			logger,
		),
		middlewares:  middlewares,
		stopSignalCh: make(chan struct{}, 1),
		stoppedCh:    make(chan struct{}, 1),
		logger:       logger,
	}

	return c
}

func (c *ConsumerSQS[T]) Consume(ctx context.Context, queueURL string, messageHandler Handler[T]) error {
	var (
		// requires some tuning to find the optimal value depending on the message processing time and visibility timeout
		bufferSize = c.cfg.HandlerWorkerPoolSize * 3

		msgs = make(chan sqstypes.Message, bufferSize)

		stopCh       = make(chan struct{})
		processErrCh = make(chan error, 1)
		pollerErrCh  chan error

		handlerFunc = newMessageHandlerFunc(messageHandler)

		processCtx, cancel = context.WithCancel(ctx)
	)

	defer func() {
		close(msgs)
		close(stopCh)
		close(processErrCh)
		cancel()
	}()

	// apply middlewares
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handlerFunc = c.middlewares[i](handlerFunc)
	}

	go func() { pollerErrCh <- c.poller.Poll(processCtx, queueURL, msgs) }()
	go func() { processErrCh <- c.processor.Process(processCtx, msgs, handlerFunc) }()

	println("Consumer started")
	select {
	case <-ctx.Done():
		c.logger.InfoContext(ctx, "context is canceled. Shutting down consumer.")
		cancel()
	}

	return nil
}

func newMessageHandlerFunc[T Message](handler Handler[T]) HandlerFunc[T] {
	return func(ctx context.Context, message T) error {
		return handler.Handle(ctx, message)
	}
}

//go:generate mockery --name=sqsConnector --output=mocks --filename=sqs_connector.go
type sqsConnector interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}
