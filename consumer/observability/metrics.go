package observability

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// SQSMetrics defines a generic and extensible interface for recording metrics.
type SQSMetrics interface {
	// Counter adds a value to a cumulative metric
	Counter(ctx context.Context, name MetricName, value int64, opts ...MetricOption)

	// Histogram records observations for calculating distributions (e.g., latency)
	Histogram(ctx context.Context, name MetricName, value float64, opts ...MetricOption)

	// Gauge records a value that can go up or down (e.g., number of active workers)
	Gauge(ctx context.Context, name MetricName, value int64, opts ...MetricOption)

	// RecordDuration is a convenience method for recording time.Duration as histogram values
	RecordDuration(ctx context.Context, name MetricName, duration time.Duration, opts ...MetricOption)
}

// otelMetrics is the concrete implementation of the SQSMetrics interface.
type otelMetrics struct {
	meter       metric.Meter
	instruments sync.Map
}

func NewMetrics(cfg *Config) SQSMetrics {
	meter := cfg.MeterProvider().Meter(
		meterName,
		metric.WithInstrumentationVersion(cfg.ServiceVersion()),
	)

	return &otelMetrics{meter: meter}
}

// Counter adds a value to a cumulative metric.
func (m *otelMetrics) Counter(ctx context.Context, name MetricName, value int64, opts ...MetricOption) {
	counter, err := m.getOrCreateCounter(name)
	if err != nil {
		otel.Handle(err)
		return
	}

	counter.Add(ctx, value, metric.WithAttributes(m.buildAttributes(opts...)...))
}

// Histogram records observations for calculating distributions.
func (m *otelMetrics) Histogram(ctx context.Context, name MetricName, value float64, opts ...MetricOption) {
	histogram, err := m.getOrCreateHistogram(name)
	if err != nil {
		otel.Handle(err)
		return
	}

	histogram.Record(ctx, value, metric.WithAttributes(m.buildAttributes(opts...)...))
}

func (m *otelMetrics) Gauge(ctx context.Context, name MetricName, value int64, opts ...MetricOption) {
	gauge, err := m.getOrCreateGauge(name)
	if err != nil {
		otel.Handle(err)
		return
	}

	gauge.Record(ctx, value, metric.WithAttributes(m.buildAttributes(opts...)...))
}

func (m *otelMetrics) RecordDuration(ctx context.Context, name MetricName, duration time.Duration, opts ...MetricOption) {
	m.Histogram(ctx, name, duration.Seconds(), opts...)
}

func (m *otelMetrics) buildAttributes(opts ...MetricOption) []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, len(opts))
	for i, opt := range opts {
		attributes[i] = opt()
	}

	return attributes
}

// instrumentResult holds the result of instrument creation for thread-safe caching
type instrumentResult[T any] struct {
	instrument T
	err        error
}

// getOrCreateInstrument is a generic helper that atomically gets or creates any
// type of metric instrument, caching it in a sync.Map.
func getOrCreateInstrument[T any](m *otelMetrics, name MetricName, factory func() (T, error)) (T, error) {
	// et or store the sync.Once initializer for this metric name
	once, _ := m.instruments.LoadOrStore(name, &sync.Once{})

	// check if it's an instrument result already
	if result, ok := once.(*instrumentResult[T]); ok {
		return result.instrument, result.err
	}

	once.(*sync.Once).Do(func() { // nolint:errcheck // sync.Once.Do never returns an error
		instrument, err := factory()
		// store the result, replacing the sync.Once object in the map
		m.instruments.Store(name, &instrumentResult[T]{
			instrument: instrument,
			err:        err,
		})
	})

	value, _ := m.instruments.Load(name)
	result := value.(*instrumentResult[T]) // nolint:errcheck // type assertion is safe here

	return result.instrument, result.err
}

func (m *otelMetrics) getOrCreateCounter(name MetricName) (metric.Int64Counter, error) {
	return getOrCreateInstrument(m, name, func() (metric.Int64Counter, error) {
		meta := metricInfo[name]
		return m.meter.Int64Counter(string(name), metric.WithDescription(meta.Description), metric.WithUnit(meta.Unit))
	})
}

func (m *otelMetrics) getOrCreateGauge(name MetricName) (metric.Int64Gauge, error) {
	return getOrCreateInstrument(m, name, func() (metric.Int64Gauge, error) {
		meta := metricInfo[name]
		return m.meter.Int64Gauge(string(name), metric.WithDescription(meta.Description), metric.WithUnit(meta.Unit))
	})
}

func (m *otelMetrics) getOrCreateHistogram(name MetricName) (metric.Float64Histogram, error) {
	return getOrCreateInstrument(m, name, func() (metric.Float64Histogram, error) {
		meta := metricInfo[name]
		return m.meter.Float64Histogram(string(name), metric.WithDescription(meta.Description), metric.WithUnit(meta.Unit))
	})
}
