package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer/observability"
	"go.opentelemetry.io/otel/propagation"
)

type processorConfig struct {
	QueueURL       string
	WorkerPoolSize int32
}

type processorSQS[T any] struct { // nolint:govet
	cfg            processorConfig
	messageAdapter MessageAdapter[T]
	acknowledger   Acknowledger
	logger         *slog.Logger
	tracer         observability.SQSTracer
	propagator     propagation.TextMapPropagator
}

func newProcessorSQS[T any](
	cfg processorConfig,
	messageAdapter MessageAdapter[T],
	acknowledger Acknowledger,
	logger *slog.Logger,
	tracer observability.SQSTracer,
	propagator propagation.TextMapPropagator,
) *processorSQS[T] {
	return &processorSQS[T]{
		cfg:            cfg,
		messageAdapter: messageAdapter,
		acknowledger:   acknowledger,
		logger:         logger,
		tracer:         tracer,
		propagator:     propagator,
	}
}

func (p *processorSQS[T]) Process(ctx context.Context, msgs <-chan sqstypes.Message, handler Handler[T]) error {
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

					p.processMessage(ctx, msg, handler)
				}
			}
		}()
	}

	wg.Wait()

	return nil
}

// processMessage handles the complete processing lifecycle for a single message
func (p *processorSQS[T]) processMessage(ctx context.Context, msg sqstypes.Message, handler Handler[T]) {
	traceCtx := observability.ExtractTraceContext(ctx, msg, p.propagator)

	msgCtx, msgSpan := p.tracer.Span(traceCtx, observability.SpanNameProcess,
		observability.WithConsumerSpanKind(),
		observability.WithQueueURL(p.cfg.QueueURL),
		observability.WithMessageID(msg.MessageId),
		observability.WithAction(observability.ActionProcess),
	)
	defer msgSpan.End()

	message, err := p.messageAdapter.Transform(msgCtx, msg)
	if err != nil {
		p.logger.ErrorContext(ctx, "error transforming message", "error", err)

		return
	}

	if err = handler.Handle(msgCtx, message); err != nil {
		// Message stays in the queue and will be processed again.
		// It will be visible again after visibility timeout.
		// If the message is not processed successfully after the maximum number of retries, it will be moved to the DLQ if configured.
		if rejErr := p.acknowledger.Reject(msgCtx, msg); rejErr != nil {
			p.logger.ErrorContext(ctx, "error rejecting message", "error", rejErr)
		}

		return
	}

	if ackErr := p.acknowledger.Ack(msgCtx, msg); ackErr != nil {
		p.logger.ErrorContext(ctx, "error acknowledging message", "error", ackErr)
	}
}
