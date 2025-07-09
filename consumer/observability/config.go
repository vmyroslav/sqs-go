package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	meterNoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

const (
	// DefaultServiceName is the default service name for observability
	DefaultServiceName = "sqs-consumer"
	// DefaultServiceVersion is the default service version for observability
	DefaultServiceVersion = "1.0.0"
)

// Config holds observability configuration
type Config struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	propagator     propagation.TextMapPropagator
	serviceName    string
	serviceVersion string
}

// Option is an interface that configures observability
type Option interface {
	apply(*Config)
}

// option is a function that configures observability
type option func(*Config)

func (o option) apply(c *Config) {
	o(c)
}

// NewConfig creates a new observability config with default values
func NewConfig(opts ...Option) *Config {
	c := &Config{
		serviceName:    DefaultServiceName,
		serviceVersion: DefaultServiceVersion,
		propagator:     otel.GetTextMapPropagator(), // use global propagator as default
		tracerProvider: traceNoop.NewTracerProvider(),
		meterProvider:  meterNoop.NewMeterProvider(),
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	return c
}

// WithTracerProvider sets the tracer provider
func WithTracerProvider(tp trace.TracerProvider) Option {
	return option(func(c *Config) {
		if tp == nil {
			tp = traceNoop.NewTracerProvider() // use noop provider if nil
		}

		c.tracerProvider = tp
	})
}

// WithMeterProvider sets the meter provider
func WithMeterProvider(mp metric.MeterProvider) Option {
	return option(func(c *Config) {
		if mp == nil {
			mp = meterNoop.NewMeterProvider() // use noop provider if nil
		}

		c.meterProvider = mp
	})
}

// WithServiceName sets the service name
func WithServiceName(name string) Option {
	return option(func(c *Config) {
		c.serviceName = name
	})
}

// WithServiceVersion sets the service version
func WithServiceVersion(version string) Option {
	return option(func(c *Config) {
		c.serviceVersion = version
	})
}

// WithPropagator sets the trace context propagator
func WithPropagator(propagator propagation.TextMapPropagator) Option {
	return option(func(c *Config) {
		c.propagator = propagator
	})
}

// TracerProvider returns the tracer provider
func (c *Config) TracerProvider() trace.TracerProvider {
	return c.tracerProvider
}

// MeterProvider returns the meter provider
func (c *Config) MeterProvider() metric.MeterProvider {
	return c.meterProvider
}

// ServiceName returns the service name
func (c *Config) ServiceName() string {
	return c.serviceName
}

// ServiceVersion returns the service version
func (c *Config) ServiceVersion() string {
	return c.serviceVersion
}

// Propagator returns the trace context propagator
func (c *Config) Propagator() propagation.TextMapPropagator {
	return c.propagator
}
