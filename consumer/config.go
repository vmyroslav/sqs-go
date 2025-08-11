package consumer

import (
	"fmt"
	"net/url"

	"github.com/vmyroslav/sqs-go/consumer/observability"
)

// Default values for consumer configuration
const (
	DefaultProcessorWorkerPoolSize = 10
	DefaultPollerWorkerPoolSize    = 2
	DefaultMaxNumberOfMessages     = 10
	DefaultWaitTimeSeconds         = 1
	DefaultVisibilityTimeout       = 30
	DefaultErrorNumberThreshold    = -1
	DefaultGracefulShutdownTimeout = 30
	DefaultAcknowledgmentStrategy  = SyncAcknowledgment
)

type WrongConfigError struct {
	Err error
}

func (e *WrongConfigError) Error() string {
	return fmt.Sprintf("wrong config: %s", e.Err)
}

type Config struct {
	Observability           *observability.Config
	QueueURL                string
	AckStrategy             AcknowledgmentStrategy
	ProcessorWorkerPoolSize int32
	PollerWorkerPoolSize    int32
	MaxNumberOfMessages     int32
	WaitTimeSeconds         int32
	VisibilityTimeout       int32
	ErrorNumberThreshold    int32
	GracefulShutdownTimeout int32
}

// Option is an interface that configures a consumer Config
type Option interface {
	apply(*Config)
}

// option is a function that configures a consumer Config
type option func(*Config)

func (o option) apply(c *Config) {
	o(c)
}

// NewConfig creates a new Config with the provided queue URL and optional configuration
func NewConfig(queueURL string, opts ...Option) (*Config, error) {
	c := &Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: DefaultProcessorWorkerPoolSize,
		PollerWorkerPoolSize:    DefaultPollerWorkerPoolSize,
		MaxNumberOfMessages:     DefaultMaxNumberOfMessages,
		WaitTimeSeconds:         DefaultWaitTimeSeconds,
		VisibilityTimeout:       DefaultVisibilityTimeout,
		ErrorNumberThreshold:    DefaultErrorNumberThreshold,
		GracefulShutdownTimeout: DefaultGracefulShutdownTimeout,
		Observability:           observability.NewConfig(), // disabled by default
		AckStrategy:             DefaultAcknowledgmentStrategy,
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	_, err := c.IsValid()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// WithProcessorWorkerPoolSize sets the processor worker pool size
func WithProcessorWorkerPoolSize(size int32) Option {
	return option(func(c *Config) {
		c.ProcessorWorkerPoolSize = size
	})
}

// WithPollerWorkerPoolSize sets the poller worker pool size
func WithPollerWorkerPoolSize(size int32) Option {
	return option(func(c *Config) {
		c.PollerWorkerPoolSize = size
	})
}

// WithMaxNumberOfMessages sets the maximum number of messages to receive in a single poll
func WithMaxNumberOfMessages(maxMessages int32) Option {
	return option(func(c *Config) {
		c.MaxNumberOfMessages = maxMessages
	})
}

// WithWaitTimeSeconds sets the wait time in seconds for long polling
func WithWaitTimeSeconds(wait int32) Option {
	return option(func(c *Config) {
		c.WaitTimeSeconds = wait
	})
}

// WithVisibilityTimeout sets the visibility timeout for messages
func WithVisibilityTimeout(timeout int32) Option {
	return option(func(c *Config) {
		c.VisibilityTimeout = timeout
	})
}

// WithErrorNumberThreshold sets the error number threshold
func WithErrorNumberThreshold(threshold int32) Option {
	return option(func(c *Config) {
		c.ErrorNumberThreshold = threshold
	})
}

// WithGracefulShutdownTimeout sets the graceful shutdown timeout
func WithGracefulShutdownTimeout(timeout int32) Option {
	return option(func(c *Config) {
		c.GracefulShutdownTimeout = timeout
	})
}

// WithObservability sets the observability configuration
func WithObservability(obs *observability.Config) Option {
	return option(func(c *Config) {
		c.Observability = obs
	})
}

// WithAcknowledgmentStrategy sets the observability configuration
func WithAcknowledgmentStrategy(as AcknowledgmentStrategy) Option {
	return option(func(c *Config) {
		c.AckStrategy = as
	})
}

func (c *Config) IsValid() (bool, error) { // nolint: cyclop
	if c.QueueURL == "" {
		return false, &WrongConfigError{Err: fmt.Errorf("queueURL is empty")}
	}

	_, err := url.ParseRequestURI(c.QueueURL)
	if err != nil {
		return false, &WrongConfigError{Err: fmt.Errorf("queueURL is not a valid URL")}
	}

	if c.ProcessorWorkerPoolSize <= 0 {
		return false, &WrongConfigError{Err: fmt.Errorf("handlerWorkerPoolSize must be greater than 0")}
	}

	if c.PollerWorkerPoolSize <= 0 {
		return false, &WrongConfigError{Err: fmt.Errorf("pollerWorkerPoolSize must be greater than 0")}
	}

	if c.MaxNumberOfMessages <= 0 || c.MaxNumberOfMessages > 10 {
		return false, &WrongConfigError{Err: fmt.Errorf("maxNumberOfMessages must be between 1 and 10")}
	}

	if c.WaitTimeSeconds < 0 || c.WaitTimeSeconds > 20 {
		return false, &WrongConfigError{Err: fmt.Errorf("waitTimeSeconds must be between 0 and 20")}
	}

	if c.VisibilityTimeout < 0 || c.VisibilityTimeout > 43200 {
		return false, &WrongConfigError{Err: fmt.Errorf("visibilityTimeout must be between 0 and 43200")}
	}

	return true, nil
}
