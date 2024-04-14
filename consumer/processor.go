package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type processorConfig struct {
	WorkerPoolSize int32
}
type processorSQS[T any] struct { // nolint:govet
	cfg            processorConfig
	messageAdapter MessageAdapter[T]
	acknowledger   acknowledger
	logger         *slog.Logger
}

func newProcessorSQS[T any](
	cfg processorConfig,
	messageAdapter MessageAdapter[T],
	acknowledger acknowledger,
	logger *slog.Logger,
) *processorSQS[T] {
	return &processorSQS[T]{
		cfg:            cfg,
		messageAdapter: messageAdapter,
		acknowledger:   acknowledger,
		logger:         logger,
	}
}

func (p *processorSQS[T]) Process(ctx context.Context, msgs <-chan sqstypes.Message, handler Handler[T]) error { // nolint: cyclop
	var (
		poolSize = int(p.cfg.WorkerPoolSize)
		wg       sync.WaitGroup
	)

	if p.cfg.WorkerPoolSize < 1 {
		return &WrongConfigError{Err: fmt.Errorf("invalid worker pool size: %d", p.cfg.WorkerPoolSize)}
	}

	for i := 0; i < poolSize; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgs:
					if !ok {
						p.logger.DebugContext(ctx, "message channel closed")

						return
					}

					message, err := p.messageAdapter.Transform(ctx, msg)
					if err != nil {
						p.logger.ErrorContext(ctx, "error transforming message", err)

						continue
					}

					if err = handler.Handle(ctx, message); err != nil { //nolint:wsl
						// Message stays in the queue and will be processed again.
						// It will be visible again after visibility timeout.
						// If the message is not processed successfully after the maximum number of retries, it will be moved to the DLQ if configured.

						if err = p.acknowledger.Reject(ctx, msg); err != nil {
							p.logger.ErrorContext(ctx, "error rejecting message", err)
						}

						continue
					}

					if err = p.acknowledger.Ack(ctx, msg); err != nil {
						p.logger.ErrorContext(ctx, "error acknowledging message", err)
					}
				}
			}
		}()
	}

	wg.Wait()

	return nil
}
