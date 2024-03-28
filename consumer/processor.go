package consumer

import (
	"context"
	"log/slog"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type processorConfig struct {
	HandlerWorkerPoolSize int32
}
type processorSQS[T Message] struct {
	cfg            processorConfig
	messageAdapter MessageAdapter[T]
	acknowledger   Confirmer
	middlewares    []Middleware[T]
	logger         *slog.Logger
}

func newProcessorSQS[T Message](
	cfg processorConfig,
	messageAdapter MessageAdapter[T],
	acknowledger Confirmer,
	middlewares []Middleware[T],
	logger *slog.Logger,
) *processorSQS[T] {
	return &processorSQS[T]{
		cfg:            cfg,
		messageAdapter: messageAdapter,
		acknowledger:   acknowledger,
		middlewares:    middlewares,
		logger:         logger,
	}
}

func (p *processorSQS[T]) Process(ctx context.Context, msgs <-chan sqstypes.Message, handler Handler[T]) error {
	var (
		processErrCh = make(chan error, 1)

		handlerFunc = newMessageHandlerFunc(handler)

		pCtx, cancel = context.WithCancel(ctx)
	)

	defer cancel()

	// apply middlewares
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		handlerFunc = p.middlewares[i](handlerFunc)
	}

	for i := int32(0); i < p.cfg.HandlerWorkerPoolSize; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgs:
					if !ok {
						return
					}

					message, err := p.messageAdapter.Transform(ctx, msg)
					if err != nil {
						p.logger.ErrorContext(ctx, "error transforming message", err)
						processErrCh <- err
						continue
					}

					if err = handlerFunc.Handle(pCtx, message); err != nil {
						processErrCh <- err
						// Message stays in the queue and will be processed again.
						// It will be visible again after visibility timeout.
						// Can return into the queue if needed by setting the visibility timeout to 0.
						// If the message is not processed successfully after the maximum number of retries, it will be moved to the DLQ if configured.
						// Exponential backoff can be implemented here by adding additional queue to the consumer.
						// Or manual processing can be implemented playing with the visibility timeout.

						// Also delay can be implemented as middleware by sleeping the goroutine based on the number of retries.

						if err = p.acknowledger.Reject(ctx, msg); err != nil {
							p.logger.ErrorContext(ctx, "error rejecting message", err)
							processErrCh <- err
						}

						continue
					}

					if err = p.acknowledger.Ack(ctx, msg); err != nil {
						p.logger.ErrorContext(ctx, "error acknowledging message", err)
						processErrCh <- err
					}
				}
			}
		}()
	}

	return <-processErrCh
}
