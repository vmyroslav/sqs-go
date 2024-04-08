package consumer

import (
	"fmt"
	"net/url"
)

type ErrWrongConfig struct {
	Err error
}

func (e *ErrWrongConfig) Error() string {
	return fmt.Sprintf("wrong config: %s", e.Err)
}

type Config struct {
	QueueURL string

	// processor configuration
	ProcessorWorkerPoolSize int32

	// poller configuration
	PollerWorkerPoolSize int32
	// MaxNumberOfMessages is the maximum number of messages to receive in a single poll request: 1-10
	MaxNumberOfMessages int32
	// WaitTimeSeconds is the duration (0 to 20 seconds) for which the call waits for a message to arrive
	WaitTimeSeconds int32
	// VisibilityTimeout is the duration (0 to 12 hours) that the received messages are hidden from subsequent retrieve requests
	VisibilityTimeout int32
	// MaxNumberOfRetries is the maximum number of retries for a message polling. -1 means infinite retries
	ErrorNumberThreshold int32
	// GracefulShutdownTimeout is the timeout for graceful shutdown.
	GracefulShutdownTimeout int32
}

func NewConfig(
	queueURL string,
	handlerWorkerPoolSize int32,
	pollerWorkerPoolSize int32,
	maxNumberOfMessages int32,
	waitTimeSeconds int32,
	visibilityTimeout int32,
	errorNumberThreshold int32,
	gracefulShutdownTimeout int32,
) (*Config, error) {
	config := &Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: handlerWorkerPoolSize,
		PollerWorkerPoolSize:    pollerWorkerPoolSize,
		MaxNumberOfMessages:     maxNumberOfMessages,
		WaitTimeSeconds:         waitTimeSeconds,
		VisibilityTimeout:       visibilityTimeout,
		ErrorNumberThreshold:    errorNumberThreshold,
		GracefulShutdownTimeout: gracefulShutdownTimeout,
	}

	_, err := config.IsValid()
	if err != nil {
		return nil, err
	}

	return config, nil
}

// NewDefaultConfig creates a new Config with default values
func NewDefaultConfig(queueURL string) *Config {
	return &Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: 10,
		PollerWorkerPoolSize:    2,
		MaxNumberOfMessages:     10,
		WaitTimeSeconds:         1,
		VisibilityTimeout:       30,
		ErrorNumberThreshold:    -1,
	}
}

func (c *Config) IsValid() (bool, error) {
	if c.QueueURL == "" {
		return false, &ErrWrongConfig{Err: fmt.Errorf("queueURL is empty")}
	}

	_, err := url.ParseRequestURI(c.QueueURL)
	if err != nil {
		return false, &ErrWrongConfig{Err: fmt.Errorf("queueURL is not a valid URL")}
	}

	if c.ProcessorWorkerPoolSize <= 0 {
		return false, &ErrWrongConfig{Err: fmt.Errorf("handlerWorkerPoolSize must be greater than 0")}
	}

	if c.PollerWorkerPoolSize <= 0 {
		return false, &ErrWrongConfig{Err: fmt.Errorf("pollerWorkerPoolSize must be greater than 0")}
	}

	if c.MaxNumberOfMessages <= 0 || c.MaxNumberOfMessages > 10 {
		return false, &ErrWrongConfig{Err: fmt.Errorf("maxNumberOfMessages must be between 1 and 10")}
	}

	if c.WaitTimeSeconds < 0 || c.WaitTimeSeconds > 20 {
		return false, &ErrWrongConfig{Err: fmt.Errorf("waitTimeSeconds must be between 0 and 20")}
	}

	if c.VisibilityTimeout < 0 || c.VisibilityTimeout > 43200 {
		return false, &ErrWrongConfig{Err: fmt.Errorf("visibilityTimeout must be between 0 and 43200")}
	}

	return true, nil
}
