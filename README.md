[![Action Status](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/vmyroslav/sqs-go/branch/main/graph/badge.svg?token=8F5APGAZT6)](https://codecov.io/gh/vmyroslav/sqs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmyroslav/sqs-go)](https://goreportcard.com/report/github.com/vmyroslav/sqs-go)
[![Godoc](https://pkg.go.dev/badge/github.com/vmyroslav/sqs-go)](https://pkg.go.dev/github.com/vmyroslav/sqs-go)

# SQS Consumer Library for Go

A type-safe Go library for consuming messages from **AWS SQS** with support for custom message adapters, middleware chains, and configurable worker pools.

## Features

- **Type-safe message processing** with Go generics
- **Flexible message adapters** - JSON adapter included, custom adapters supported
- **Middleware support** for cross-cutting concerns (logging, metrics, error handling)
- **Configurable worker pools** for polling and processing
- **Built-in error handling** with configurable thresholds

## Installation

```bash
go get github.com/vmyroslav/sqs-go
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/vmyroslav/sqs-go/consumer"
)

type MyMessage struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

func main() {
	ctx := context.Background()
	
	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}
	
	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := "https://sqs.region.amazonaws.com/account/queue-name"
	
	// Create JSON message adapter
	adapter := consumer.NewJSONMessageAdapter[MyMessage]()
	
	// Configure consumer
	cfg := consumer.NewDefaultConfig(queueURL)
	sqsConsumer := consumer.NewSQSConsumer[MyMessage](
		cfg, 
		sqsClient, 
		adapter, 
		nil,  // middleware
		nil,  // logger
	)
	
	// Define message handler
	handler := consumer.HandlerFunc[MyMessage](func(ctx context.Context, msg MyMessage) error {
		log.Printf("Processing message: %+v", msg)
		return nil
	})
	
	// Start consuming
	if err = sqsConsumer.Consume(ctx, queueURL, handler); err != nil {
		log.Fatal(err)
	}
}
```

## Configuration

The `Config` struct provides customization options:

```go
config := consumer.Config{
    QueueURL:                "https://sqs.region.amazonaws.com/account/queue-name",
    ProcessorWorkerPoolSize: 10,    // Number of worker goroutines for processing messages
    PollerWorkerPoolSize:    2,     // Number of worker goroutines for polling SQS
    MaxNumberOfMessages:     10,    // Max messages to receive in a single request (1-10)
    WaitTimeSeconds:         20,    // Long polling wait time (0-20 seconds)
    VisibilityTimeout:       30,    // Message visibility timeout in seconds
    ErrorNumberThreshold:    5,     // Max consecutive errors before stopping
    GracefulShutdownTimeout: 30,    // Graceful shutdown timeout in seconds
}
```

### Default Configuration

Use `NewDefaultConfig(queueURL)` for production-ready defaults:
- 10 processor workers
- 2 poller workers
- 20-second long polling
- 30-second visibility timeout
- Graceful shutdown enabled

## Message Adapters

### JSON Adapter

For JSON messages, use the built-in JSON adapter:

```go
type UserEvent struct {
    UserID string `json:"user_id"`
    Action string `json:"action"`
}

adapter := consumer.NewJSONMessageAdapter[UserEvent]()
```

### Custom Adapter

Implement the `MessageAdapter` interface for custom message formats:

```go
type CustomAdapter struct{}

func (a CustomAdapter) Transform(ctx context.Context, msg sqstypes.Message) (MyType, error) {
    // Custom message transformation logic
    return MyType{
        ID:   *msg.MessageId,
        Body: *msg.Body,
    }, nil
}

// Or use MessageAdapterFunc for simple cases
adapter := consumer.MessageAdapterFunc[MyType](func(ctx context.Context, msg sqstypes.Message) (MyType, error) {
    return MyType{
        ID:   *msg.MessageId, 
        Body: *msg.Body,
    }, nil
})
```

## Middleware

Middleware enables cross-cutting concerns like logging, metrics, and error handling:

```go
// Logging middleware
func loggingMiddleware[T any](logger *slog.Logger) consumer.Middleware[T] {
    return func(next consumer.HandlerFunc[T]) consumer.HandlerFunc[T] {
        return func(ctx context.Context, msg T) error {
            logger.Info("Processing message", "message", msg)
            err := next.Handle(ctx, msg)
            if err != nil {
                logger.Error("Failed to process message", "error", err)
            }
            return err
        }
    }
}

// Apply middleware
middlewares := []consumer.Middleware[MyMessage]{
    loggingMiddleware[MyMessage](logger),
    metricsMiddleware[MyMessage](),
}

sqsConsumer := consumer.NewSQSConsumer[MyMessage](
    config, sqsClient, adapter, middlewares, logger,
)
```

## Advanced Examples

For more examples, see the `/examples` directory.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License.