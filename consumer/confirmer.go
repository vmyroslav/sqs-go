package consumer

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

type immediateAcknowledger struct {
	sqsClient sqsConnector
	queueURL  string
}

func newImmediateAcknowledger(url string, client sqsConnector) *immediateAcknowledger {
	return &immediateAcknowledger{
		sqsClient: client,
		queueURL:  url,
	}
}

func (a *immediateAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &a.queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (a *immediateAcknowledger) newVisibilityTimeoutInput(receiptHandle *string) *sqs.ChangeMessageVisibilityInput {
	return &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(a.queueURL),
		ReceiptHandle:     receiptHandle,
		VisibilityTimeout: 0,
	}
}

func (a *immediateAcknowledger) Reject(ctx context.Context, msg sqstypes.Message) error {
	if _, err := a.sqsClient.ChangeMessageVisibility(ctx, a.newVisibilityTimeoutInput(msg.ReceiptHandle)); err != nil {
		return fmt.Errorf("change message visibility: msg: %s: %w", aws.ToString(msg.ReceiptHandle), err)
	}

	return nil
}

type exponentialAcknowledger struct {
	sqsClient sqsConnector
	queueURL  string
}

func newExponentialAcknowledger(url string, client sqsConnector) *exponentialAcknowledger {
	return &exponentialAcknowledger{
		sqsClient: client,
		queueURL:  url,
	}
}

func (a *exponentialAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &a.queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (a *exponentialAcknowledger) calculateVisibilityTimeout(msg sqstypes.Message) int32 {
	baseDelay := 100 * time.Millisecond
	maxDelay := 2000 * time.Millisecond

	receiveCount := int32(1)

	if attr, exists := msg.Attributes["ApproximateReceiveCount"]; exists {
		if count, err := strconv.ParseInt(attr, 10, 32); err == nil {
			receiveCount = int32(count)
		}
	}

	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(receiveCount-1)))

	return int32(max(delay, maxDelay).Seconds())
}

func (a *exponentialAcknowledger) Reject(ctx context.Context, msg sqstypes.Message) error {
	_, err := a.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(a.queueURL),
		ReceiptHandle:     msg.ReceiptHandle,
		VisibilityTimeout: a.calculateVisibilityTimeout(msg),
	})
	if err != nil {
		return fmt.Errorf("change message visibility with exponential backoff: msg: %s: %w", aws.ToString(msg.ReceiptHandle), err)
	}

	return nil
}

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
