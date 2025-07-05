package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	traceapi "go.opentelemetry.io/otel/trace"
)

const (
	testQueueURL   = "test-queue-url"
	testStatusAttr = "status"
)

// Test message type
type TestMessage struct {
	ID      string
	Content string
}

func TestProcessingMiddleware(t *testing.T) {
	t.Parallel()
	t.Run("Successful message processing", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithMeterProvider(meterProvider),
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)

		tracer := NewTracer(cfg)
		metrics := NewMetrics(cfg)
		queueURL := testQueueURL

		middleware := Middleware[TestMessage](tracer, metrics, queueURL)

		// handler that succeeds
		handlerCalled := false
		handler := func(_ context.Context, msg TestMessage) error {
			handlerCalled = true

			assert.Equal(t, "test-msg", msg.ID)
			assert.Equal(t, "test-content", msg.Content)

			return nil
		}

		wrappedHandler := middleware(handler)

		ctx := context.Background()
		msg := TestMessage{ID: "test-msg", Content: "test-content"}
		err := wrappedHandler(ctx, msg)

		require.NoError(t, err)
		assert.True(t, handlerCalled)

		// verify span was created
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, SpanNameHandle, spanData.Name)
		assert.Equal(t, codes.Ok, spanData.Status.Code)
		assert.Equal(t, traceapi.SpanKindInternal, spanData.SpanKind)

		// check attributes
		attrMap := make(map[string]string)
		for _, attr := range spanData.Attributes {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, "aws-sqs", attrMap["messaging.system"])
		assert.Equal(t, "consume", attrMap["messaging.sqs.action"])

		// verify metrics were recorded
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		require.NoError(t, err)

		foundCounter := checkMessageMetric(t, rm, queueURL, testStatusSuccess)

		assert.True(t, foundCounter)
	})

	t.Run("Failed message processing", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithMeterProvider(meterProvider),
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)

		tracer := NewTracer(cfg)
		metrics := NewMetrics(cfg)
		queueURL := testQueueURL

		middleware := Middleware[TestMessage](tracer, metrics, queueURL)

		// handler that fails
		testError := errors.New("processing failed")
		handlerCalled := false
		handler := func(_ context.Context, _ TestMessage) error {
			handlerCalled = true
			return testError
		}

		wrappedHandler := middleware(handler)

		ctx := context.Background()
		msg := TestMessage{ID: "test-msg", Content: "test-content"}
		err := wrappedHandler(ctx, msg)

		require.Error(t, err)
		assert.Equal(t, testError, err)
		assert.True(t, handlerCalled)

		// verify span was created with error
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, SpanNameHandle, spanData.Name)
		assert.Equal(t, codes.Error, spanData.Status.Code)

		// check for error event
		require.Len(t, spanData.Events, 1)
		event := spanData.Events[0]
		assert.Equal(t, "exception", event.Name)

		// verify metrics were recorded with error status
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		require.NoError(t, err)

		// check counter metric
		var foundCounter bool

		for _, scope := range rm.ScopeMetrics {
			for _, metricData := range scope.Metrics {
				if metricData.Name == string(MetricMessages) {
					foundCounter = true

					if sum, ok := metricData.Data.(metricdata.Sum[int64]); ok {
						require.Len(t, sum.DataPoints, 1)
						dp := sum.DataPoints[0]
						assert.Equal(t, int64(1), dp.Value)

						// Check status attribute
						attrs := dp.Attributes
						statusFound := false

						for _, attr := range attrs.ToSlice() {
							if string(attr.Key) == "status" && attr.Value.AsString() == testStatusError {
								statusFound = true
							}
						}

						assert.True(t, statusFound)
					}
				}
			}
		}

		assert.True(t, foundCounter)
	})

	t.Run("Measures processing duration", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithMeterProvider(meterProvider),
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)

		tracer := NewTracer(cfg)
		metrics := NewMetrics(cfg)
		queueURL := testQueueURL

		middleware := Middleware[TestMessage](tracer, metrics, queueURL)

		// handler that takes some time
		processingDelay := 100 * time.Millisecond
		handler := func(_ context.Context, _ TestMessage) error {
			time.Sleep(processingDelay)
			return nil
		}

		wrappedHandler := middleware(handler)

		ctx := context.Background()
		msg := TestMessage{ID: "test-msg", Content: "test-content"}
		start := time.Now()
		err := wrappedHandler(ctx, msg)
		totalDuration := time.Since(start)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, totalDuration, processingDelay)

		// verify duration metric was recorded
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		require.NoError(t, err)

		var foundHistogram bool

		for _, scope := range rm.ScopeMetrics {
			for _, metricData := range scope.Metrics {
				if metricData.Name == string(MetricProcessingDuration) {
					foundHistogram = true

					if hist, ok := metricData.Data.(metricdata.Histogram[float64]); ok {
						require.Len(t, hist.DataPoints, 1)
						dp := hist.DataPoints[0]
						assert.Equal(t, uint64(1), dp.Count)
						assert.Greater(t, dp.Sum, processingDelay.Seconds())
					}
				}
			}
		}

		assert.True(t, foundHistogram)
	})
}

func TestMiddlewareSpanAttributes(t *testing.T) {
	t.Parallel()
	t.Run("Span created with correct attributes", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tracerProvider := trace.NewTracerProvider(trace.WithSyncer(exporter))

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)

		tracer := NewTracer(cfg)

		metrics := NewMetrics(NewConfig())

		queueURL := testQueueURL

		middleware := Middleware[TestMessage](tracer, metrics, queueURL)

		handler := func(_ context.Context, _ TestMessage) error {
			return nil
		}

		wrappedHandler := middleware(handler)

		ctx := context.Background()
		msg := TestMessage{ID: "test-msg", Content: "test-content"}
		err := wrappedHandler(ctx, msg)

		require.NoError(t, err)

		// verify span attributes
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		spanData := spans[0]
		assert.Equal(t, SpanNameHandle, spanData.Name)
		assert.Equal(t, traceapi.SpanKindInternal, spanData.SpanKind)

		// check attributes
		attrMap := make(map[string]string)
		for _, attr := range spanData.Attributes {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, "aws-sqs", attrMap["messaging.system"])
		assert.Equal(t, "consume", attrMap["messaging.sqs.action"])
	})
}

func TestMiddlewareGenericType(t *testing.T) {
	t.Parallel()
	t.Run("Works with different message types", func(t *testing.T) {
		t.Parallel()
		// Test with string type
		stringMiddleware := Middleware[string](
			NewTracer(NewConfig()),
			NewMetrics(NewConfig()),
			"test-queue",
		)

		stringHandler := func(_ context.Context, msg string) error {
			assert.Equal(t, "test message", msg)
			return nil
		}

		wrappedStringHandler := stringMiddleware(stringHandler)
		err := wrappedStringHandler(context.Background(), "test message")
		require.NoError(t, err)

		// test with custom struct type
		type CustomMessage struct {
			Data string
		}

		customMiddleware := Middleware[CustomMessage](
			NewTracer(NewConfig()),
			NewMetrics(NewConfig()),
			"test-queue",
		)

		customHandler := func(_ context.Context, msg CustomMessage) error {
			assert.Equal(t, "custom data", msg.Data)
			return nil
		}

		wrappedCustomHandler := customMiddleware(customHandler)
		err = wrappedCustomHandler(context.Background(), CustomMessage{Data: "custom data"})
		require.NoError(t, err)
	})
}

// checkMessageMetric is a helper function to reduce complexity in metric verification
func checkMessageMetric(t *testing.T, rm *metricdata.ResourceMetrics, expectedQueueURL, expectedStatus string) bool {
	t.Helper()

	for _, scope := range rm.ScopeMetrics {
		for _, metricData := range scope.Metrics {
			if metricData.Name == string(MetricMessages) {
				return validateMessageSum(t, metricData, expectedQueueURL, expectedStatus)
			}
		}
	}

	return false
}

// validateMessageSum validates the sum metric data for message counters
func validateMessageSum(t *testing.T, metricData metricdata.Metrics, expectedQueueURL, expectedStatus string) bool {
	t.Helper()

	sum, ok := metricData.Data.(metricdata.Sum[int64])
	if !ok {
		return false
	}

	require.Len(t, sum.DataPoints, 1)
	dp := sum.DataPoints[0]
	assert.Equal(t, int64(1), dp.Value)

	validateMessageAttributes(t, dp.Attributes, expectedQueueURL, expectedStatus)

	return true
}

// validateMessageAttributes validates the attributes on message metrics
func validateMessageAttributes(t *testing.T, attrs attribute.Set, expectedQueueURL, expectedStatus string) {
	t.Helper()

	queueFound := false
	statusFound := false

	for _, attr := range attrs.ToSlice() {
		if string(attr.Key) == "queue_url" && attr.Value.AsString() == expectedQueueURL {
			queueFound = true
		}

		if string(attr.Key) == testStatusAttr && attr.Value.AsString() == expectedStatus {
			statusFound = true
		}
	}

	assert.True(t, queueFound)
	assert.True(t, statusFound)
}
