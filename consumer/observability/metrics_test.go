package observability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	testStatusSuccess = "success"
	testStatusError   = "error"
)

func TestSQSMetrics_ValueVerification(t *testing.T) {
	t.Parallel()

	t.Run("Counter value verification", func(t *testing.T) {
		t.Parallel()
		// create a metric reader to capture metrics data
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		defer func() { _ = provider.Shutdown(context.Background()) }()

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		metrics := NewMetrics(cfg)
		ctx := context.Background()
		// record counter values
		metrics.Counter(ctx, MetricMessages, 5, WithStatus(testStatusSuccess))
		metrics.Counter(ctx, MetricMessages, 3, WithStatus(testStatusSuccess))
		metrics.Counter(ctx, MetricMessages, 2, WithStatus(testStatusError))

		// collect metrics
		rm := &metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, rm)
		require.NoError(t, err)

		// find our counter metric
		var (
			foundCounter             bool
			successCount, errorCount int64
		)

		for _, scope := range rm.ScopeMetrics {
			for _, metricData := range scope.Metrics {
				if metricData.Name == string(MetricMessages) {
					foundCounter = true

					if sum, ok := metricData.Data.(metricdata.Sum[int64]); ok {
						for _, dp := range sum.DataPoints {
							// check attributes to distinguish success vs error
							statusAttr := findAttribute(dp.Attributes, "status")
							switch statusAttr {
							case testStatusSuccess:
								successCount = dp.Value
							case testStatusError:
								errorCount = dp.Value
							}
						}
					}
				}
			}
		}

		assert.True(t, foundCounter, "Should find the counter metric")
		assert.Equal(t, int64(8), successCount, "Success counter should be 5+3=8")
		assert.Equal(t, int64(2), errorCount, "Error counter should be 2")
	})

	t.Run("Gauge value verification", func(t *testing.T) {
		t.Parallel()
		// create a metric reader to capture metrics data
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		defer func() { _ = provider.Shutdown(context.Background()) }()

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		metrics := NewMetrics(cfg)
		ctx := context.Background()
		// record gauge values (last value wins)
		metrics.Gauge(ctx, MetricActiveWorkers, 10, WithProcessType("poller"))
		metrics.Gauge(ctx, MetricActiveWorkers, 5, WithProcessType("processor"))
		metrics.Gauge(ctx, MetricActiveWorkers, 0, WithProcessType("poller")) // This should overwrite the first value

		// collect metrics
		rm := &metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, rm)
		require.NoError(t, err)

		// find our gauge metric
		var (
			foundGauge                      bool
			pollerWorkers, processorWorkers int64
		)

		for _, scope := range rm.ScopeMetrics {
			for _, metricData := range scope.Metrics {
				if metricData.Name == string(MetricActiveWorkers) {
					foundGauge = true

					if gauge, ok := metricData.Data.(metricdata.Gauge[int64]); ok {
						for _, dp := range gauge.DataPoints {
							// Check attributes to distinguish poller vs processor
							processTypeAttr := findAttribute(dp.Attributes, "process_type")
							switch processTypeAttr {
							case "poller":
								pollerWorkers = dp.Value
							case "processor":
								processorWorkers = dp.Value
							}
						}
					}
				}
			}
		}

		assert.True(t, foundGauge, "Should find the gauge metric")
		assert.Equal(t, int64(0), pollerWorkers, "Poller workers should be 0 (last value)")
		assert.Equal(t, int64(5), processorWorkers, "Processor workers should be 5")
	})

	t.Run("Histogram value verification", func(t *testing.T) {
		t.Parallel()
		// create a metric reader to capture metrics data
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		defer func() { _ = provider.Shutdown(context.Background()) }()

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		metrics := NewMetrics(cfg)
		ctx := context.Background()
		// record histogram values
		metrics.Histogram(ctx, MetricProcessingDuration, 1.5, WithQueueURLMetric("test-queue"))
		metrics.Histogram(ctx, MetricProcessingDuration, 2.5, WithQueueURLMetric("test-queue"))
		metrics.RecordDuration(ctx, MetricProcessingDuration, 500*time.Millisecond, WithQueueURLMetric("test-queue"))

		// collect metrics
		rm := &metricdata.ResourceMetrics{}
		err := reader.Collect(ctx, rm)
		require.NoError(t, err)

		// find our histogram metric
		var (
			foundHistogram bool
			count          uint64
			sum            float64
		)

		for _, scope := range rm.ScopeMetrics {
			for _, metricData := range scope.Metrics {
				if metricData.Name == string(MetricProcessingDuration) {
					foundHistogram = true

					if hist, ok := metricData.Data.(metricdata.Histogram[float64]); ok {
						for _, dp := range hist.DataPoints {
							queueAttr := findAttribute(dp.Attributes, "queue_url")
							if queueAttr == "test-queue" {
								count = dp.Count
								sum = dp.Sum

								break
							}
						}
					}
				}
			}
		}

		assert.True(t, foundHistogram, "Should find the histogram metric")
		assert.Equal(t, uint64(3), count, "Should have 3 histogram observations")
		// Sum should be 1.5 + 2.5 + 0.5 = 4.5
		assert.InDelta(t, 4.5, sum, 0.01, "Sum should be approximately 4.5")
	})
}

// Helper function to find an attribute value by key
func findAttribute(attrs attribute.Set, key string) string {
	iter := attrs.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}

	return ""
}

func TestSQSMetrics_Disabled(t *testing.T) {
	t.Parallel()

	cfg := NewConfig() // disabled by default - uses noop providers
	metrics := NewMetrics(cfg)

	_, ok := metrics.(*otelMetrics)
	assert.True(t, ok, "Should return otelMetrics with noop providers when disabled")

	// these should not panic and should be effectively no-ops via OpenTelemetry noop providers
	ctx := context.Background()
	metrics.Counter(ctx, MetricMessages, 1)
	metrics.Histogram(ctx, MetricProcessingDuration, 1.0)
	metrics.Gauge(ctx, MetricActiveWorkers, 5)
	metrics.RecordDuration(ctx, MetricPollingDuration, time.Second)
}

func TestOtelMetrics_InstrumentCreation(t *testing.T) {
	t.Parallel()

	t.Run("Counter creation and caching", func(t *testing.T) {
		t.Parallel()

		provider := sdkmetric.NewMeterProvider()

		t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

		// first call should create the counter
		counter1, err1 := m.getOrCreateCounter(MetricMessages)
		require.NoError(t, err1)
		require.NotNil(t, counter1)

		// second call should return the same counter (cached)
		counter2, err2 := m.getOrCreateCounter(MetricMessages)
		require.NoError(t, err2)
		assert.Equal(t, counter1, counter2, "Should return the same cached counter")
	})

	t.Run("Gauge creation and caching", func(t *testing.T) {
		t.Parallel()

		provider := sdkmetric.NewMeterProvider()

		t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

		gauge1, err1 := m.getOrCreateGauge(MetricActiveWorkers)
		require.NoError(t, err1)
		require.NotNil(t, gauge1)

		gauge2, err2 := m.getOrCreateGauge(MetricActiveWorkers)
		require.NoError(t, err2)
		assert.Equal(t, gauge1, gauge2, "Should return the same cached gauge")
	})

	t.Run("Histogram creation and caching", func(t *testing.T) {
		t.Parallel()

		provider := sdkmetric.NewMeterProvider()

		t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

		histogram1, err1 := m.getOrCreateHistogram(MetricProcessingDuration)
		require.NoError(t, err1)
		require.NotNil(t, histogram1)

		histogram2, err2 := m.getOrCreateHistogram(MetricProcessingDuration)
		require.NoError(t, err2)
		assert.Equal(t, histogram1, histogram2, "Should return the same cached histogram")
	})
}

func TestOtelMetrics_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("Concurrent counter access", func(t *testing.T) {
		t.Parallel()

		provider := sdkmetric.NewMeterProvider()

		t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

		const (
			numGoroutines = 100
			numIterations = 10
		)

		var wg sync.WaitGroup

		results := make([]metric.Int64Counter, numGoroutines)
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func(index int) {
				defer wg.Done()

				for j := 0; j < numIterations; j++ {
					counter, err := m.getOrCreateCounter(MetricPollingRequests)
					results[index] = counter
					errors[index] = err
				}
			}(i)
		}

		wg.Wait()

		// all results should be the same counter instance
		var firstCounter metric.Int64Counter

		for i, counter := range results {
			require.NoError(t, errors[i], "Should not have errors")

			if i == 0 {
				firstCounter = counter
			} else {
				assert.Equal(t, firstCounter, counter, "All goroutines should get the same counter instance")
			}
		}
	})

	t.Run("Concurrent different metrics", func(t *testing.T) {
		t.Parallel()

		provider := sdkmetric.NewMeterProvider()

		t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

		cfg := NewConfig(
			WithMeterProvider(provider),
			WithServiceVersion("test-1.0.0"),
		)

		m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

		const numGoroutines = 100

		var wg sync.WaitGroup

		ctx := context.Background()

		// Test concurrent access to different metric types
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func(index int) {
				defer wg.Done()
				// Each goroutine performs different metric operations
				m.Counter(ctx, MetricMessages, int64(index), WithStatus("test"))
				m.Histogram(ctx, MetricProcessingDuration, float64(index), WithQueueURLMetric("test"))
				m.Gauge(ctx, MetricActiveWorkers, int64(index), WithProcessType("test"))
			}(i)
		}

		wg.Wait()
	})
}

func TestOtelMetrics_FactoryCallCount(t *testing.T) {
	t.Parallel()

	provider := sdkmetric.NewMeterProvider()

	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	cfg := NewConfig(
		WithMeterProvider(provider),
		WithServiceVersion("test-1.0.0"),
	)

	m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

	t.Run("Factory called exactly once", func(t *testing.T) {
		t.Parallel()

		var callCount int32

		const numGoroutines = 50

		// custom getOrCreateInstrument call with a counting factory
		factoryFunc := func() (metric.Int64Counter, error) {
			callCount++
			return m.meter.Int64Counter("test-counter")
		}

		var wg sync.WaitGroup

		results := make([]metric.Int64Counter, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func(index int) {
				defer wg.Done()

				counter, _ := getOrCreateInstrument(m, MetricBufferSize, factoryFunc)
				results[index] = counter
			}(i)
		}

		wg.Wait()

		// factory should have been called exactly once
		assert.Equal(t, int32(1), callCount, "Factory function should be called exactly once")

		// all results should be the same
		for i := 1; i < numGoroutines; i++ {
			assert.Equal(t, results[0], results[i], "All results should be the same counter")
		}
	})
}

func TestOtelMetrics_AttributeBuilding(t *testing.T) {
	t.Parallel()

	provider := sdkmetric.NewMeterProvider()

	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	cfg := NewConfig(
		WithMeterProvider(provider),
		WithServiceVersion("test-1.0.0"),
	)

	m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test

	t.Run("Build attributes correctly", func(t *testing.T) {
		t.Parallel()

		opts := []MetricOption{
			WithStatus("success"),
			WithQueueURLMetric("test-queue"),
			WithProcessType("poller"),
			WithWorkerID("worker-1"),
		}

		attrs := m.buildAttributes(opts...)
		require.Len(t, attrs, 4)

		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, "success", attrMap["status"])
		assert.Equal(t, "test-queue", attrMap["queue_url"])
		assert.Equal(t, "poller", attrMap["process_type"])
		assert.Equal(t, "worker-1", attrMap["worker_id"])
	})

	t.Run("Empty attributes", func(t *testing.T) {
		t.Parallel()

		attrs := m.buildAttributes()
		assert.Empty(t, attrs)
	})
}

// Benchmark to ensure the refactored implementation performs well
func BenchmarkOtelMetrics_ConcurrentAccess(b *testing.B) {
	provider := sdkmetric.NewMeterProvider()

	defer func() { _ = provider.Shutdown(context.Background()) }()

	cfg := NewConfig(
		WithMeterProvider(provider),
		WithServiceVersion("bench-1.0.0"),
	)

	m := NewMetrics(cfg).(*otelMetrics) // nolint:errcheck // type assertion is safe in test
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Counter(ctx, MetricMessages, 1, WithStatus("success"))
			m.Histogram(ctx, MetricProcessingDuration, 0.1, WithQueueURLMetric("bench-queue"))
			m.Gauge(ctx, MetricActiveWorkers, 10, WithProcessType("poller"))
		}
	})
}
