package consumer

import "fmt"

type ErrWrongConfig struct {
	Err error
}

func (e *ErrWrongConfig) Error() string {
	return fmt.Sprintf("wrong config: %s", e.Err)
}

type Config struct {
	QueueURL string

	HandlerWorkerPoolSize int32

	// poller configuration
	PollerWorkerPoolSize int32
	MaxNumberOfMessages  int32
	WaitTimeSeconds      int32
	VisibilityTimeout    int32
	MaxNumberOfRetries   int32
	ErrorNumberThreshold int32
}

func NewConfig(
	AWSRegion string,
	queueURL string,
	handlerWorkerPoolSize int32,
	pollerWorkerPoolSize int32,
	maxNumberOfMessages int32,
	waitTimeSeconds int32,
	visibilityTimeout int32,
	maxNumberOfRetries int32,
	errorNumberThreshold int32,
) (*Config, error) {
	if handlerWorkerPoolSize <= 0 {
		return nil, fmt.Errorf("handlerWorkerPoolSize must be greater than 0")
	}

	if pollerWorkerPoolSize <= 0 {
		return nil, fmt.Errorf("pollerWorkerPoolSize must be greater than 0")
	}

	if maxNumberOfMessages <= 0 {
		return nil, fmt.Errorf("maxNumberOfMessages must be greater than 0")
	}

	if waitTimeSeconds <= 0 {
		return nil, fmt.Errorf("waitTimeSeconds must be greater than 0")
	}

	if visibilityTimeout <= 0 {
		return nil, fmt.Errorf("visibilityTimeout must be greater than 0")
	}

	if maxNumberOfRetries <= 0 {
		return nil, fmt.Errorf("maxNumberOfRetries must be greater than 0")
	}

	return &Config{
		QueueURL:              queueURL,
		HandlerWorkerPoolSize: handlerWorkerPoolSize,
		PollerWorkerPoolSize:  pollerWorkerPoolSize,
		MaxNumberOfMessages:   maxNumberOfMessages,
		WaitTimeSeconds:       waitTimeSeconds,
		VisibilityTimeout:     visibilityTimeout,
		MaxNumberOfRetries:    maxNumberOfRetries,
		ErrorNumberThreshold:  errorNumberThreshold,
	}, nil
}
