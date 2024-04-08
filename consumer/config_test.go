package consumer

import (
	"testing"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.IsValid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.IsValid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Config.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
