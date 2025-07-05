package observability

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/vmyroslav/sqs-go/consumer"
)

const (
	// SpanNameProcess represents the message processing operation span name
	SpanNameProcess = "sqs.message.process"
	// SpanNameTransform represents the message transformation span name
	SpanNameTransform = "sqs.message.transform"
	// SpanNameHandle represents the message handling span name
	SpanNameHandle = "sqs.handler.handle"
	// SpanNameAck represents the message acknowledgment span name
	SpanNameAck = "sqs.message.ack"
)

// Action represents a type-safe SQS operation for observability
type Action string

const (
	// ActionAck represents message acknowledgment action
	ActionAck Action = "ack"
	// ActionReject represents message rejection action
	ActionReject Action = "reject"
	// ActionProcess represents message processing action
	ActionProcess Action = "process"
	// ActionTransform represents message transformation action
	ActionTransform Action = "transform"
	// ActionConsume represents message consumption action
	ActionConsume Action = "consume"
)

// WithMessageID adds the SQS message ID as a span attribute.
// It follows OpenTelemetry semantic conventions for messaging.
func WithMessageID(messageID *string) SpanOption {
	if messageID == nil || *messageID == "" {
		return trace.WithAttributes() // return a no-op option if the ID is nil or empty
	}

	return trace.WithAttributes(attribute.String("messaging.message.id", *messageID))
}

// WithQueueURL adds the SQS queue URL as a span attribute.
func WithQueueURL(queueURL string) SpanOption {
	if queueURL == "" {
		return trace.WithAttributes() // return a no-op option if the URL is empty
	}

	return trace.WithAttributes(attribute.String("sqs.queue.url", queueURL))
}

// WithAction adds a custom action attribute to a span using type-safe Action constants.
func WithAction(action Action) SpanOption {
	return trace.WithAttributes(attribute.String("messaging.sqs.action", string(action)))
}

// WithConsumerSpanKind sets the span kind to Consumer, typically used for message processing.
func WithConsumerSpanKind() SpanOption {
	return trace.WithSpanKind(trace.SpanKindConsumer)
}

// WithClientSpanKind sets the span kind to Client, typically used for SQS API calls.
func WithClientSpanKind() SpanOption {
	return trace.WithSpanKind(trace.SpanKindClient)
}

// WithInternalSpanKind sets the span kind to Internal, typically used for internal processing.
func WithInternalSpanKind() SpanOption {
	return trace.WithSpanKind(trace.SpanKindInternal)
}
