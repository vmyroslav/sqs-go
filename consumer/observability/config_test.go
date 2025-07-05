package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()
	t.Run("Creates config with default values", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig()

		assert.NotNil(t, cfg)
		assert.Equal(t, DefaultServiceName, cfg.ServiceName())
		assert.Equal(t, DefaultServiceVersion, cfg.ServiceVersion())
		assert.NotNil(t, cfg.TracerProvider())
		assert.NotNil(t, cfg.MeterProvider())
		assert.NotNil(t, cfg.Propagator())

		// default propagator should be the global one
		assert.Equal(t, otel.GetTextMapPropagator(), cfg.Propagator())

		// default providers should be noop
		_, isNoopTracer := cfg.TracerProvider().(noop.TracerProvider)
		assert.True(t, isNoopTracer)
	})

	t.Run("Creates config with custom options", func(t *testing.T) {
		t.Parallel()

		customTracerProvider := trace.NewTracerProvider()
		customMeterProvider := sdkmetric.NewMeterProvider()
		customPropagator := propagation.TraceContext{}

		cfg := NewConfig(
			WithTracerProvider(customTracerProvider),
			WithMeterProvider(customMeterProvider),
			WithServiceName("custom-service"),
			WithServiceVersion("2.0.0"),
			WithPropagator(customPropagator),
		)

		assert.NotNil(t, cfg)
		assert.Equal(t, "custom-service", cfg.ServiceName())
		assert.Equal(t, "2.0.0", cfg.ServiceVersion())
		assert.Equal(t, customTracerProvider, cfg.TracerProvider())
		assert.Equal(t, customMeterProvider, cfg.MeterProvider())
		assert.Equal(t, customPropagator, cfg.Propagator())
	})
}

func TestWithTracerProvider(t *testing.T) {
	t.Parallel()
	t.Run("Sets tracer provider", func(t *testing.T) {
		t.Parallel()

		customProvider := trace.NewTracerProvider()
		cfg := NewConfig(WithTracerProvider(customProvider))

		assert.Equal(t, customProvider, cfg.TracerProvider())
	})

	t.Run("Overwrites default tracer provider", func(t *testing.T) {
		t.Parallel()

		customProvider := trace.NewTracerProvider()
		cfg := NewConfig()

		// default should be noop
		_, isNoop := cfg.TracerProvider().(noop.TracerProvider)
		assert.True(t, isNoop)

		// apply custom provider
		WithTracerProvider(customProvider).apply(cfg)

		assert.Equal(t, customProvider, cfg.TracerProvider())
	})
}

func TestWithMeterProvider(t *testing.T) {
	t.Parallel()
	t.Run("Sets meter provider", func(t *testing.T) {
		t.Parallel()

		customProvider := sdkmetric.NewMeterProvider()
		cfg := NewConfig(WithMeterProvider(customProvider))

		assert.Equal(t, customProvider, cfg.MeterProvider())
	})

	t.Run("Overwrites default meter provider", func(t *testing.T) {
		t.Parallel()

		customProvider := sdkmetric.NewMeterProvider()
		cfg := NewConfig()

		// apply custom provider
		WithMeterProvider(customProvider).apply(cfg)

		assert.Equal(t, customProvider, cfg.MeterProvider())
	})
}

func TestWithServiceName(t *testing.T) {
	t.Parallel()
	t.Run("Sets service name", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceName("test-service"))

		assert.Equal(t, "test-service", cfg.ServiceName())
	})

	t.Run("Overwrites default service name", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig()

		// default should be DefaultServiceName
		assert.Equal(t, DefaultServiceName, cfg.ServiceName())

		// apply custom name
		WithServiceName("custom-service").apply(cfg)

		assert.Equal(t, "custom-service", cfg.ServiceName())
	})

	t.Run("Handles empty service name", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceName(""))

		assert.Empty(t, cfg.ServiceName())
	})
}

func TestWithServiceVersion(t *testing.T) {
	t.Parallel()
	t.Run("Sets service version", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceVersion("3.0.0"))

		assert.Equal(t, "3.0.0", cfg.ServiceVersion())
	})

	t.Run("Overwrites default service version", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig()

		// default should be DefaultServiceVersion
		assert.Equal(t, DefaultServiceVersion, cfg.ServiceVersion())

		// apply custom version
		WithServiceVersion("2.5.0").apply(cfg)

		assert.Equal(t, "2.5.0", cfg.ServiceVersion())
	})

	t.Run("Handles empty service version", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceVersion(""))

		assert.Empty(t, cfg.ServiceVersion())
	})
}

func TestWithPropagator(t *testing.T) {
	t.Parallel()
	t.Run("Sets propagator", func(t *testing.T) {
		t.Parallel()

		customPropagator := propagation.TraceContext{}
		cfg := NewConfig(WithPropagator(customPropagator))

		assert.Equal(t, customPropagator, cfg.Propagator())
	})

	t.Run("Overwrites default propagator", func(t *testing.T) {
		t.Parallel()

		customPropagator := propagation.TraceContext{}

		cfg := NewConfig()

		// default should be global propagator
		assert.Equal(t, otel.GetTextMapPropagator(), cfg.Propagator())

		// apply custom propagator
		WithPropagator(customPropagator).apply(cfg)

		assert.Equal(t, customPropagator, cfg.Propagator())
	})
}

func TestConfigGetters(t *testing.T) {
	t.Parallel()
	t.Run("TracerProvider getter", func(t *testing.T) {
		t.Parallel()

		customProvider := trace.NewTracerProvider()
		cfg := NewConfig(WithTracerProvider(customProvider))

		assert.Equal(t, customProvider, cfg.TracerProvider())
	})

	t.Run("MeterProvider getter", func(t *testing.T) {
		t.Parallel()

		customProvider := sdkmetric.NewMeterProvider()
		cfg := NewConfig(WithMeterProvider(customProvider))

		assert.Equal(t, customProvider, cfg.MeterProvider())
	})

	t.Run("ServiceName getter", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceName("test-service"))

		assert.Equal(t, "test-service", cfg.ServiceName())
	})

	t.Run("ServiceVersion getter", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(WithServiceVersion("1.5.0"))

		assert.Equal(t, "1.5.0", cfg.ServiceVersion())
	})

	t.Run("Propagator getter", func(t *testing.T) {
		t.Parallel()

		customPropagator := propagation.TraceContext{}
		cfg := NewConfig(WithPropagator(customPropagator))

		assert.Equal(t, customPropagator, cfg.Propagator())
	})
}

func TestOptions(t *testing.T) {
	t.Parallel()
	t.Run("Options implement Option interface", func(t *testing.T) {
		t.Parallel()

		opts := []Option{
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracerProvider(trace.NewTracerProvider()),
			WithMeterProvider(sdkmetric.NewMeterProvider()),
			WithPropagator(propagation.TraceContext{}),
		}

		cfg := NewConfig(opts...)

		assert.NotNil(t, cfg)
		assert.Equal(t, "test", cfg.ServiceName())
		assert.Equal(t, "1.0.0", cfg.ServiceVersion())
	})
}

func TestOptionApply(t *testing.T) {
	t.Parallel()
	t.Run("option function applies correctly", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{
			serviceName:    "initial",
			serviceVersion: "0.1.0",
		}

		opt := option(func(c *Config) {
			c.serviceName = "modified"
			c.serviceVersion = "0.2.0"
		})

		opt.apply(cfg)

		assert.Equal(t, "modified", cfg.serviceName)
		assert.Equal(t, "0.2.0", cfg.serviceVersion)
	})
}

func TestMultipleOptions(t *testing.T) {
	t.Parallel()
	t.Run("Multiple options are applied in order", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(
			WithServiceName("first"),
			WithServiceVersion("1.0.0"),
			WithServiceName("second"),   // This should overwrite the first
			WithServiceVersion("2.0.0"), // This should overwrite the first
		)

		assert.Equal(t, "second", cfg.ServiceName())
		assert.Equal(t, "2.0.0", cfg.ServiceVersion())
	})

	t.Run("Options can be mixed", func(t *testing.T) {
		t.Parallel()

		tracerProvider := trace.NewTracerProvider()
		meterProvider := sdkmetric.NewMeterProvider()
		propagator := propagation.TraceContext{}

		cfg := NewConfig(
			WithServiceName("mixed-service"),
			WithTracerProvider(tracerProvider),
			WithServiceVersion("3.0.0"),
			WithMeterProvider(meterProvider),
			WithPropagator(propagator),
		)

		assert.Equal(t, "mixed-service", cfg.ServiceName())
		assert.Equal(t, "3.0.0", cfg.ServiceVersion())
		assert.Equal(t, tracerProvider, cfg.TracerProvider())
		assert.Equal(t, meterProvider, cfg.MeterProvider())
		assert.Equal(t, propagator, cfg.Propagator())
	})
}

func TestDefaultConstants(t *testing.T) {
	t.Parallel()
	t.Run("Default constants are correct", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "sqs-consumer", DefaultServiceName)
		assert.Equal(t, "1.0.0", DefaultServiceVersion)
	})
}

func TestConfigNilSafety(t *testing.T) {
	t.Parallel()
	t.Run("Config handles nil providers gracefully", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{}

		// Apply a nil tracer provider (should not panic and should set to noop)
		WithTracerProvider(nil).apply(cfg)
		_, isNoopTracer := cfg.TracerProvider().(noop.TracerProvider)
		assert.True(t, isNoopTracer, "Nil tracer provider should be converted to noop provider")

		// Apply a nil meter provider (should not panic and should set to noop)
		WithMeterProvider(nil).apply(cfg)
		assert.NotNil(t, cfg.MeterProvider(), "Nil meter provider should be converted to noop provider")

		// Apply a nil propagator (should not panic and should remain nil)
		WithPropagator(nil).apply(cfg)
		assert.Nil(t, cfg.propagator, "Nil propagator should remain nil")
	})

	t.Run("NewConfig with nil providers", func(t *testing.T) {
		t.Parallel()

		cfg := NewConfig(
			WithTracerProvider(nil),
			WithMeterProvider(nil),
			WithPropagator(nil),
		)

		// All nil providers should be converted to noop providers except propagator
		_, isNoopTracer := cfg.TracerProvider().(noop.TracerProvider)
		assert.True(t, isNoopTracer, "Nil tracer provider should be converted to noop provider")

		assert.NotNil(t, cfg.MeterProvider(), "Nil meter provider should be converted to noop provider")

		assert.Nil(t, cfg.Propagator(), "Nil propagator should remain nil")
	})
}

func TestConfigIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Config can be used to create observability components", func(t *testing.T) {
		t.Parallel()

		tracerProvider := trace.NewTracerProvider()
		meterProvider := sdkmetric.NewMeterProvider()

		cfg := NewConfig(
			WithTracerProvider(tracerProvider),
			WithMeterProvider(meterProvider),
			WithServiceName("integration-test"),
			WithServiceVersion("1.0.0"),
		)

		tracer := NewTracer(cfg)
		metrics := NewMetrics(cfg)

		assert.NotNil(t, tracer)
		assert.NotNil(t, metrics)

		// should be able to use the components
		ctx := context.Background()
		spanCtx, span := tracer.Span(ctx, "test-span")
		assert.NotNil(t, spanCtx)
		assert.NotNil(t, span)
		span.End()

		metrics.Counter(ctx, MetricMessages, 1)
		metrics.Gauge(ctx, MetricActiveWorkers, 5)
		metrics.Histogram(ctx, MetricProcessingDuration, 0.5)
	})
}
