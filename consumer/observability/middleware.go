package observability

import (
	"context"
	"time"
)

// Middleware creates a comprehensive processing middleware that handles tracing and metrics
func Middleware[T any](tracer SQSTracer, metrics SQSMetrics, queueURL string) func(next func(ctx context.Context, msg T) error) func(ctx context.Context, msg T) error {
	return processingMiddleware[T](tracer, metrics, queueURL)
}

// processingMiddleware creates a comprehensive middleware that handles the complete message processing pipeline
// This includes span creation and metrics recording for the handler
func processingMiddleware[T any](tracer SQSTracer, metrics SQSMetrics, queueURL string) func(next func(ctx context.Context, msg T) error) func(ctx context.Context, msg T) error {
	return func(next func(ctx context.Context, msg T) error) func(ctx context.Context, msg T) error {
		return func(ctx context.Context, msg T) error {
			// create handler span - this is a child of the processing span
			handlerCtx, handlerSpan := tracer.Span(ctx, SpanNameHandle,
				WithInternalSpanKind(),
				WithAction(ActionConsume),
			)
			defer handlerSpan.End()

			start := time.Now()

			err := next(handlerCtx, msg)

			duration := time.Since(start)

			status := RecordSpanResult(handlerSpan, err)

			metrics.RecordDuration(ctx, MetricProcessingDuration, duration,
				WithQueueURLMetric(queueURL),
				WithStatus(status),
			)

			metrics.Counter(ctx, MetricMessages, 1,
				WithQueueURLMetric(queueURL),
				WithStatus(status),
			)

			return err
		}
	}
}
