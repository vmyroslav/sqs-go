package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	_, err := NewConfig(
		"http://localhost:4566/000000000000/queue",
		10,
		2,
		10,
		1,
		30,
		-1,
		30,
	)
	if err != nil {
		t.Errorf("NewConfig() error = %v", err)
	}
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
		"",
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	)

	if err == nil {
		t.Errorf("NewConfig() error = %v, wantErr %v", err, true)
	}

	assert.ErrorIs(t, err, &WrongConfigError{})
}

func TestNewDefaultConfig(t *testing.T) {
	queueURL := "http://localhost:4566/000000000000/queue"
	config := NewDefaultConfig(queueURL)

	if config.QueueURL != queueURL {
		t.Errorf("NewDefaultConfig() QueueURL = %v, want %v", config.QueueURL, queueURL)
	}

	valid, err := config.IsValid()
	require.NoError(t, err)
	assert.True(t, valid)
}
