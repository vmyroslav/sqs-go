package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer/observability"
)

// syncAcknowledger is used to ack/reject messages.
// An example of very simple sync implementation is provided without any error handling on our side.
type syncAcknowledger struct {
	sqsClient sqsConnector
	queueURL  string
}

func newSyncAcknowledger(queueURL string, sqsClient sqsConnector) *syncAcknowledger {
	return &syncAcknowledger{sqsClient: sqsClient, queueURL: queueURL}
}

func (a *syncAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &a.queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (a *syncAcknowledger) Reject(_ context.Context, _ sqstypes.Message) error { return nil }

// observableAcknowledger wraps an acknowledger with observability instrumentation
type observableAcknowledger struct {
	acknowledger acknowledger
	tracer       observability.SQSTracer
	metrics      observability.SQSMetrics
	queueURL     string
}

// newObservableAcknowledger creates a new observability-decorated acknowledger
func newObservableAcknowledger(
	acknowledger acknowledger,
	tracer observability.SQSTracer,
	metrics observability.SQSMetrics,
	queueURL string,
) *observableAcknowledger {
	return &observableAcknowledger{
		acknowledger: acknowledger,
		tracer:       tracer,
		metrics:      metrics,
		queueURL:     queueURL,
	}
}

// Ack wraps acknowledgment with observability instrumentation
func (d *observableAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	return d.instrumentAckOperation(ctx, observability.ActionAck, func(ctx context.Context) error {
		return d.acknowledger.Ack(ctx, msg)
	})
}

// Reject wraps rejection with observability instrumentation
func (d *observableAcknowledger) Reject(ctx context.Context, msg sqstypes.Message) error {
	return d.instrumentAckOperation(ctx, observability.ActionReject, func(ctx context.Context) error {
		return d.acknowledger.Reject(ctx, msg)
	})
}

func (d *observableAcknowledger) instrumentAckOperation(
	ctx context.Context,
	action observability.Action,
	operation func(context.Context) error,
) error {
	ackCtx, ackSpan := d.tracer.Span(ctx, observability.SpanNameAck,
		observability.WithClientSpanKind(),
		observability.WithQueueURL(d.queueURL),
		observability.WithAction(action),
	)
	defer ackSpan.End()

	start := time.Now()

	err := operation(ackCtx)

	duration := time.Since(start)

	status := observability.RecordSpanResult(ackSpan, err)

	d.metrics.RecordDuration(ctx, observability.MetricAcknowledgmentDuration, duration,
		observability.WithQueueURLMetric(d.queueURL),
	)

	d.metrics.Counter(ctx, observability.MetricMessages, 1,
		observability.WithQueueURLMetric(d.queueURL),
		observability.WithActionMetric(action),
		observability.WithStatus(status),
	)

	return err
}
