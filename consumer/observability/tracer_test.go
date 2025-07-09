package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	traceapi "go.opentelemetry.io/otel/trace"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

func TestNewTracer(t *testing.T) {
	t.Parallel()
	t.Run("With enabled tracer provider", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)

		otTracer, ok := tracer.(*otelTracer)
		require.True(t, ok)
		assert.NotNil(t, otTracer.tracer)
	})

	t.Run("With disabled tracer provider", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig()

		tracer := NewTracer(cfg)

		otTracer, ok := tracer.(*otelTracer)
		require.True(t, ok)
		assert.NotNil(t, otTracer.tracer)
	})
}

func TestOtelTracer_Span(t *testing.T) {
	t.Parallel()

	t.Run("Creates span with default attributes", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		spanCtx, span := tracer.Span(ctx, "test-span")

		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, "test-span", spanData.Name)

		// Check default attributes
		attrs := spanData.Attributes
		found := false

		for _, attr := range attrs {
			if attr.Key == "messaging.system" && attr.Value.AsString() == "aws-sqs" {
				found = true
				break
			}
		}

		assert.True(t, found, "Should have default messaging.system attribute")
	})

	t.Run("Creates span with custom attributes", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		spanCtx, span := tracer.Span(ctx, "test-span-with-attrs",
			WithMessageID(toPtr("test-msg-id")),
			WithQueueURL("test-queue-url"),
			WithAction(ActionProcess),
		)

		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, "test-span-with-attrs", spanData.Name)

		// Check attributes
		attrMap := make(map[string]string)
		for _, attr := range spanData.Attributes {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, "aws-sqs", attrMap["messaging.system"])
		assert.Equal(t, "test-msg-id", attrMap["messaging.message.id"])
		assert.Equal(t, "test-queue-url", attrMap["sqs.queue.url"])
		assert.Equal(t, "process", attrMap["messaging.sqs.action"])
	})

	t.Run("Creates span with custom span kind", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		spanCtx, span := tracer.Span(ctx, "test-span-consumer",
			WithConsumerSpanKind(),
		)

		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, traceapi.SpanKindConsumer, spanData.SpanKind)
	})
}

func TestOtelTracer_Disabled(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	tracer := NewTracer(cfg)

	ctx := context.Background()

	spanCtx, span := tracer.Span(ctx, "test-span")

	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)

	// Should not panic with noop tracer
	span.End()
}

func TestRecordError(t *testing.T) {
	t.Parallel()

	t.Run("Records error on span", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		_, span := tracer.Span(ctx, "test-error-span")

		testErr := errors.New("test error")
		RecordError(span, testErr)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, codes.Error, spanData.Status.Code)
		assert.Equal(t, "test error", spanData.Status.Description)

		// Check for error event
		require.Len(t, spanData.Events, 1)
		event := spanData.Events[0]
		assert.Equal(t, "exception", event.Name)
	})

	t.Run("Does nothing for nil error", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		_, span := tracer.Span(ctx, "test-nil-error-span")

		RecordError(span, nil)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, codes.Unset, spanData.Status.Code)
		assert.Empty(t, spanData.Events)
	})
}

func TestSetSpanSuccess(t *testing.T) {
	t.Parallel()

	t.Run("Sets span to success status", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		_, span := tracer.Span(ctx, "test-success-span")

		SetSpanSuccess(span)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, codes.Ok, spanData.Status.Code)
	})
}

func TestRecordSpanResult(t *testing.T) {
	t.Parallel()

	t.Run("Records error and returns error status", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		_, span := tracer.Span(ctx, "test-error-result-span")

		testErr := errors.New("test error")
		status := RecordSpanResult(span, testErr)

		assert.Equal(t, "error", status)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, codes.Error, spanData.Status.Code)
		assert.Equal(t, "test error", spanData.Status.Description)
	})

	t.Run("Records success and returns success status", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceVersion("test-1.0.0"),
		)

		tracer := NewTracer(cfg)
		ctx := context.Background()

		_, span := tracer.Span(ctx, "test-success-result-span")

		status := RecordSpanResult(span, nil)

		assert.Equal(t, "success", status)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, codes.Ok, spanData.Status.Code)
	})
}

func TestTraceOptions(t *testing.T) {
	t.Parallel()
	t.Run("WithMessageID", func(t *testing.T) {
		t.Parallel()
		t.Run("With valid message ID", func(t *testing.T) {
			t.Parallel()

			opt := WithMessageID(toPtr("test-msg-id"))
			// Should not panic when used
			assert.NotPanics(t, func() { _ = opt })
		})

		t.Run("With nil message ID", func(t *testing.T) {
			t.Parallel()

			opt := WithMessageID(nil)
			// Should return a no-op option that doesn't panic
			assert.NotPanics(t, func() { _ = opt })
		})

		t.Run("With empty message ID", func(t *testing.T) {
			t.Parallel()

			opt := WithMessageID(toPtr(""))
			// Should return a no-op option that doesn't panic
			assert.NotPanics(t, func() { _ = opt })
		})
	})

	t.Run("WithQueueURL", func(t *testing.T) {
		t.Parallel()
		t.Run("With valid queue URL", func(t *testing.T) {
			t.Parallel()

			opt := WithQueueURL("test-queue-url")
			// Should not panic when used
			assert.NotPanics(t, func() { _ = opt })
		})

		t.Run("With empty queue URL", func(t *testing.T) {
			t.Parallel()

			opt := WithQueueURL("")
			// Should return a no-op option that doesn't panic
			assert.NotPanics(t, func() { _ = opt })
		})
	})

	t.Run("WithAction", func(t *testing.T) {
		t.Parallel()

		actions := []Action{ActionAck, ActionReject, ActionProcess, ActionTransform, ActionConsume}
		for _, action := range actions {
			opt := WithAction(action)

			assert.NotPanics(t, func() { _ = opt })
		}
	})

	t.Run("WithSpanKind options", func(t *testing.T) {
		t.Parallel()
		assert.NotPanics(t, func() { _ = WithConsumerSpanKind() })
		assert.NotPanics(t, func() { _ = WithClientSpanKind() })
		assert.NotPanics(t, func() { _ = WithInternalSpanKind() })
	})
}

func TestSQSTracerInterface(t *testing.T) {
	t.Parallel()

	var tracer SQSTracer

	t.Run("otelTracer implements SQSTracer", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig()
		tracer = NewTracer(cfg)

		assert.NotNil(t, tracer)

		ctx := context.Background()
		spanCtx, span := tracer.Span(ctx, "test")

		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)

		span.End()
	})
}

func TestOtelTracer_WithDisabledProvider(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(WithTracerProvider(traceNoop.NewTracerProvider()))
	tracer := NewTracer(cfg)

	ctx := context.Background()

	t.Run("Works with noop tracer", func(t *testing.T) {
		t.Parallel()

		spanCtx, span := tracer.Span(ctx, "test-span")

		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)

		// Should not panic
		RecordError(span, errors.New("test error"))
		SetSpanSuccess(span)
		RecordSpanResult(span, errors.New("test error"))

		span.End()
	})
}
