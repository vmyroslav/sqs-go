package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer/observability"
)

// JSONMessageAdapter is a message adapter for json messages
type JSONMessageAdapter[T any] struct{}

func NewJSONMessageAdapter[T any]() *JSONMessageAdapter[T] {
	return &JSONMessageAdapter[T]{}
}

func (a *JSONMessageAdapter[T]) Transform(_ context.Context, msg sqstypes.Message) (T, error) {
	var m T

	if err := json.Unmarshal([]byte(*msg.Body), &m); err != nil {
		return m, fmt.Errorf("failed to unmarshal message body: %w", err)
	}

	return m, nil
}

type DummyAdapter[T sqstypes.Message] struct{}

func NewDummyAdapter[T sqstypes.Message]() *DummyAdapter[T] {
	return &DummyAdapter[T]{}
}

func (a *DummyAdapter[T]) Transform(_ context.Context, msg sqstypes.Message) (T, error) {
	return T(msg), nil
}

// observableMessageAdapter wraps a MessageAdapter with observability instrumentation
type observableMessageAdapter[T any] struct {
	adapter  MessageAdapter[T]
	tracer   observability.SQSTracer
	metrics  observability.SQSMetrics
	queueURL string
}

// newObservableMessageAdapter creates a new observability-decorated message adapter
func newObservableMessageAdapter[T any](
	adapter MessageAdapter[T],
	tracer observability.SQSTracer,
	metrics observability.SQSMetrics,
	queueURL string,
) *observableMessageAdapter[T] {
	return &observableMessageAdapter[T]{
		adapter:  adapter,
		tracer:   tracer,
		metrics:  metrics,
		queueURL: queueURL,
	}
}

// Transform wraps the transformation with observability instrumentation
func (d *observableMessageAdapter[T]) Transform(ctx context.Context, msg sqstypes.Message) (T, error) {
	tCtx, transformSpan := d.tracer.Span(ctx, observability.SpanNameTransform,
		observability.WithInternalSpanKind(),
		observability.WithQueueURL(d.queueURL),
	)
	defer transformSpan.End()

	start := time.Now()

	result, err := d.adapter.Transform(tCtx, msg)

	duration := time.Since(start)

	status := observability.RecordSpanResult(transformSpan, err)

	d.metrics.RecordDuration(ctx, observability.MetricProcessingDuration, duration,
		observability.WithQueueURLMetric(d.queueURL),
		observability.WithActionMetric(observability.ActionTransform),
		observability.WithStatus(status),
	)

	d.metrics.Counter(ctx, observability.MetricMessages, 1,
		observability.WithQueueURLMetric(d.queueURL),
		observability.WithActionMetric(observability.ActionTransform),
		observability.WithStatus(status),
	)

	return result, err //nolint:wrapcheck
}
