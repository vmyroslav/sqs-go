package consumer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmyroslav/sqs-go/consumer/observability"
)

func TestNewConfig(t *testing.T) {
	t.Run("NewConfig with functional options", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithProcessorWorkerPoolSize(10),
			WithPollerWorkerPoolSize(2),
			WithMaxNumberOfMessages(10),
			WithWaitTimeSeconds(1),
			WithVisibilityTimeout(30),
			WithErrorNumberThreshold(-1),
			WithGracefulShutdownTimeout(30),
			WithAcknowledgmentStrategy(ImmediateAcknowledgment),
		)

		require.NoError(t, err)
		assert.Equal(t, "http://localhost:4566/000000000000/queue", config.QueueURL)
		assert.Equal(t, int32(10), config.ProcessorWorkerPoolSize)
		assert.Equal(t, int32(2), config.PollerWorkerPoolSize)
		assert.Equal(t, int32(10), config.MaxNumberOfMessages)
		assert.Equal(t, int32(1), config.WaitTimeSeconds)
		assert.Equal(t, int32(30), config.VisibilityTimeout)
		assert.Equal(t, int32(-1), config.ErrorNumberThreshold)
		assert.Equal(t, int32(30), config.GracefulShutdownTimeout)
		assert.Equal(t, ImmediateAcknowledgment, config.AckStrategy)
		assert.NotNil(t, config.Observability)
	})

	t.Run("NewConfig with default values", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue")

		require.NoError(t, err)
		assert.Equal(t, "http://localhost:4566/000000000000/queue", config.QueueURL)
		assert.Equal(t, int32(DefaultProcessorWorkerPoolSize), config.ProcessorWorkerPoolSize)
		assert.Equal(t, int32(DefaultPollerWorkerPoolSize), config.PollerWorkerPoolSize)
		assert.Equal(t, int32(DefaultMaxNumberOfMessages), config.MaxNumberOfMessages)
		assert.Equal(t, int32(DefaultWaitTimeSeconds), config.WaitTimeSeconds)
		assert.Equal(t, int32(DefaultVisibilityTimeout), config.VisibilityTimeout)
		assert.Equal(t, int32(DefaultErrorNumberThreshold), config.ErrorNumberThreshold)
		assert.Equal(t, int32(DefaultGracefulShutdownTimeout), config.GracefulShutdownTimeout)
		assert.Equal(t, SyncAcknowledgment, config.AckStrategy)

		assert.NotNil(t, config.Observability)
	})
}

func TestConfig_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		want    bool
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 10,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Invalid Config - Empty QueueURL",
			config: &Config{
				QueueURL:                "",
				ProcessorWorkerPoolSize: 10,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid QueueURL",
			config: &Config{
				QueueURL:                "invalid-url",
				ProcessorWorkerPoolSize: 10,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid ProcessorWorkerPoolSize",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 0,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid PollerWorkerPoolSize",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 1,
				PollerWorkerPoolSize:    0,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid MaxNumberOfMessages",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 2,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     100,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid WaitTimeSeconds",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 2,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         10000,
				VisibilityTimeout:       30,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid VisibilityTimeout Negative",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 2,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       -1,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid VisibilityTimeout Too Large",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 2,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       50000,
				ErrorNumberThreshold:    -1,
				GracefulShutdownTimeout: 30,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Invalid Config - Invalid AcknowledgmentStrategy",
			config: &Config{
				QueueURL:                "http://localhost:4566/000000000000/queue",
				ProcessorWorkerPoolSize: 2,
				PollerWorkerPoolSize:    2,
				MaxNumberOfMessages:     10,
				WaitTimeSeconds:         1,
				VisibilityTimeout:       1,
				ErrorNumberThreshold:    1,
				GracefulShutdownTimeout: 30,
				AckStrategy:             AcknowledgmentStrategy("unknown"),
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.IsValid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.IsValid() error = %v, wantErr %v", err, tt.wantErr)
				assert.ErrorIs(t, err, &WrongConfigError{})

				return
			}

			if got != tt.want {
				t.Errorf("Config.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewInvalidConfig(t *testing.T) {
	_, err := NewConfig(
		"",                             // invalid empty queue URL
		WithProcessorWorkerPoolSize(0), // invalid worker pool size
		WithPollerWorkerPoolSize(0),    // invalid poller pool size
		WithMaxNumberOfMessages(0),     // invalid max messages
		WithWaitTimeSeconds(-1),        // invalid wait time
		WithVisibilityTimeout(-1),      // invalid visibility timeout
	)
	if err == nil {
		t.Errorf("NewConfig() error = %v, wantErr %v", err, true)
	}

	var wrongConfigErr *WrongConfigError
	if !errors.As(err, &wrongConfigErr) {
		t.Errorf("expected error of type WrongConfigError, got %T", err)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	queueURL := "http://localhost:4566/000000000000/queue"
	config, err := NewConfig(queueURL) // No options means default values
	require.NoError(t, err)

	if config.QueueURL != queueURL {
		t.Errorf("NewConfig() QueueURL = %v, want %v", config.QueueURL, queueURL)
	}

	assert.Equal(t, int32(DefaultProcessorWorkerPoolSize), config.ProcessorWorkerPoolSize)
	assert.Equal(t, int32(DefaultPollerWorkerPoolSize), config.PollerWorkerPoolSize)
	assert.Equal(t, int32(DefaultMaxNumberOfMessages), config.MaxNumberOfMessages)
	assert.Equal(t, int32(DefaultWaitTimeSeconds), config.WaitTimeSeconds)
	assert.Equal(t, int32(DefaultVisibilityTimeout), config.VisibilityTimeout)
	assert.Equal(t, int32(DefaultErrorNumberThreshold), config.ErrorNumberThreshold)
	assert.Equal(t, int32(DefaultGracefulShutdownTimeout), config.GracefulShutdownTimeout)
	assert.NotNil(t, config.Observability)

	valid, err := config.IsValid()
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestFunctionalOptions(t *testing.T) {
	t.Run("WithProcessorWorkerPoolSize", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithProcessorWorkerPoolSize(20),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(20), config.ProcessorWorkerPoolSize)
	})

	t.Run("WithPollerWorkerPoolSize", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithPollerWorkerPoolSize(5),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(5), config.PollerWorkerPoolSize)
	})

	t.Run("WithMaxNumberOfMessages", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithMaxNumberOfMessages(8),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(8), config.MaxNumberOfMessages)
	})

	t.Run("WithWaitTimeSeconds", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithWaitTimeSeconds(15),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(15), config.WaitTimeSeconds)
	})

	t.Run("WithVisibilityTimeout", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithVisibilityTimeout(60),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(60), config.VisibilityTimeout)
	})

	t.Run("WithErrorNumberThreshold", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithErrorNumberThreshold(10),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(10), config.ErrorNumberThreshold)
	})

	t.Run("WithGracefulShutdownTimeout", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithGracefulShutdownTimeout(60),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(60), config.GracefulShutdownTimeout)
	})

	t.Run("WithObservability", func(t *testing.T) {
		observabilityConfig := observability.NewConfig(
			observability.WithServiceName("custom-service"),
		)
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithObservability(observabilityConfig),
		)
		require.NoError(t, err)
		assert.Equal(t, observabilityConfig, config.Observability)
		assert.Equal(t, "custom-service", config.Observability.ServiceName())
	})

	t.Run("Multiple options", func(t *testing.T) {
		config, err := NewConfig("http://localhost:4566/000000000000/queue",
			WithProcessorWorkerPoolSize(15),
			WithPollerWorkerPoolSize(3),
			WithMaxNumberOfMessages(5),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(15), config.ProcessorWorkerPoolSize)
		assert.Equal(t, int32(3), config.PollerWorkerPoolSize)
		assert.Equal(t, int32(5), config.MaxNumberOfMessages)
		// Ensure defaults are used for unspecified options
		assert.Equal(t, int32(DefaultWaitTimeSeconds), config.WaitTimeSeconds)
		assert.Equal(t, int32(DefaultVisibilityTimeout), config.VisibilityTimeout)
	})

	t.Run("Full configuration example", func(t *testing.T) {
		// Example of a fully configured consumer using the new functional options
		observabilityConfig := observability.NewConfig(
			observability.WithServiceName("my-sqs-consumer"),
			observability.WithServiceVersion("2.0.0"),
		)

		config, err := NewConfig("https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
			WithProcessorWorkerPoolSize(20),
			WithPollerWorkerPoolSize(5),
			WithMaxNumberOfMessages(8),
			WithWaitTimeSeconds(10),
			WithVisibilityTimeout(120),
			WithErrorNumberThreshold(5),
			WithGracefulShutdownTimeout(60),
			WithObservability(observabilityConfig),
		)
		require.NoError(t, err)

		assert.Equal(t, "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue", config.QueueURL)
		assert.Equal(t, int32(20), config.ProcessorWorkerPoolSize)
		assert.Equal(t, int32(5), config.PollerWorkerPoolSize)
		assert.Equal(t, int32(8), config.MaxNumberOfMessages)
		assert.Equal(t, int32(10), config.WaitTimeSeconds)
		assert.Equal(t, int32(120), config.VisibilityTimeout)
		assert.Equal(t, int32(5), config.ErrorNumberThreshold)
		assert.Equal(t, int32(60), config.GracefulShutdownTimeout)
		assert.Equal(t, observabilityConfig, config.Observability)
		assert.Equal(t, "my-sqs-consumer", config.Observability.ServiceName())
		assert.Equal(t, "2.0.0", config.Observability.ServiceVersion())

		valid, err := config.IsValid()
		require.NoError(t, err)
		assert.True(t, valid)
	})
}
