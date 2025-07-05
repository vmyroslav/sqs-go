package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SQSTracer defines the interface for creating observability spans.
type SQSTracer interface {
	// Span starts a new trace span with a required name and optional details.
	Span(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, trace.Span)
}

// SpanOption is an alias for OpenTelemetry's span start options, allowing
// seamless integration with the OpenTelemetry ecosystem.
type SpanOption = trace.SpanStartOption

type otelTracer struct {
	tracer trace.Tracer
}

func NewTracer(cfg *Config) SQSTracer {
	tracer := cfg.TracerProvider().Tracer(
		tracerName,
		trace.WithInstrumentationVersion(cfg.ServiceVersion()),
	)

	return &otelTracer{tracer: tracer}
}

// Span starts a new trace span with a required name and optional details.
// Automatically includes default SQS messaging system attributes.
func (t *otelTracer) Span(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, trace.Span) {
	// Automatically include default attributes for all SQS spans
	defaultOpts := []SpanOption{
		trace.WithAttributes(attribute.String("messaging.system", "aws-sqs")),
	}
	opts = append(defaultOpts, opts...)

	return t.tracer.Start(ctx, spanName, opts...)
}

var _ SQSTracer = (*otelTracer)(nil)

func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func SetSpanSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "success")
}

// RecordSpanResult records span status and returns corresponding status string.
// This eliminates the common pattern of checking err, calling RecordError/SetSpanSuccess,
// and setting status string.
//
// Returns:
//   - "error" if err != nil (and records the error on span)
//   - "success" if err == nil (and sets span to success status)
func RecordSpanResult(span trace.Span, err error) string {
	if err != nil {
		RecordError(span, err)
		return "error"
	}

	SetSpanSuccess(span)

	return "success" // TODO: add enum type for status
}
