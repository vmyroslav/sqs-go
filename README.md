[![Action Status](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/vmyroslav/sqs-go/branch/main/graph/badge.svg?token=8F5APGAZT6)](https://codecov.io/gh/vmyroslav/sqs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmyroslav/sqs-go)](https://goreportcard.com/report/github.com/vmyroslav/sqs-go)
[![Godoc](https://pkg.go.dev/badge/github.com/vmyroslav/sqs-go)](https://pkg.go.dev/github.com/vmyroslav/sqs-go)

# SQS Consumer Library for Go

A type-safe Go library for consuming messages from **AWS SQS** with support for custom message adapters, middleware chains, configurable worker pools, and OpenTelemetry observability.

## Features

- **Type-safe message processing** with Go generics
- **Flexible message adapters** - JSON adapter included, custom adapters supported
- **Middleware support** for cross-cutting concerns (logging, metrics, error handling)
- **Configurable worker pools** for polling and processing
- **Built-in error handling** with configurable thresholds
- **OpenTelemetry observability** - distributed tracing and metrics with automatic instrumentation

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
	cfg, err := consumer.NewConfig(queueURL)
	if err != nil {
		log.Fatal(err)
	}
	
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

Use functional options to configure the consumer:

```go
config, err := consumer.NewConfig(queueURL,
    consumer.WithProcessorWorkerPoolSize(10),    // Number of worker goroutines for processing messages
    consumer.WithPollerWorkerPoolSize(2),        // Number of worker goroutines for polling SQS
    consumer.WithMaxNumberOfMessages(10),        // Max messages to receive in a single request (1-10)
    consumer.WithWaitTimeSeconds(20),            // Long polling wait time (0-20 seconds)
    consumer.WithVisibilityTimeout(30),          // Message visibility timeout in seconds
    consumer.WithErrorNumberThreshold(5),        // Max consecutive errors before stopping
    consumer.WithGracefulShutdownTimeout(30),    // Graceful shutdown timeout in seconds
)
if err != nil {
    log.Fatal(err)
}
```

### Default Configuration

Use `NewConfig(queueURL)` without options for defaults:
- 10 processor workers
- 2 poller workers
- 1-second long polling
- 30-second visibility timeout
- Graceful shutdown enabled

## Observability

Enable OpenTelemetry-based observability with distributed tracing and metrics:

### Basic Observability Setup

```go
import (
    "github.com/vmyroslav/sqs-go/consumer"
    "github.com/vmyroslav/sqs-go/consumer/observability"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/metric"
)

// Setup OpenTelemetry providers
traceExporter, _ := jaeger.New(jaeger.WithCollectorEndpoint())
tracerProvider := trace.NewTracerProvider(trace.WithBatcher(traceExporter))

metricExporter, _ := prometheus.New()
meterProvider := metric.NewMeterProvider(metric.WithReader(metricExporter))

// Configure observability
obsConfig := observability.NewConfig(
    observability.WithServiceName("my-sqs-consumer"),
    observability.WithServiceVersion("1.0.0"),
    observability.WithTracerProvider(tracerProvider),
    observability.WithMeterProvider(meterProvider),
)

// Create consumer with observability
config, err := consumer.NewConfig(queueURL,
    consumer.WithObservability(obsConfig),
)
```

### Available Metrics

The library automatically collects these metrics:
- **Message counters**: `sqs_messages_total` (success/error status)
- **Processing duration**: `sqs_processing_duration_seconds`
- **Polling duration**: `sqs_polling_duration_seconds`
- **Active workers**: `sqs_active_workers` (poller/processor)
- **Buffer size**: `sqs_buffer_size`
- **Acknowledgment duration**: `sqs_acknowledgment_duration_seconds`

### Distributed Tracing

Traces are automatically created for:
- **Poll operations**: SQS message polling
- **Message processing**: End-to-end message handling
- **Message transformation**: Adapter operations
- **Acknowledgment**: Message ack/reject operations

Context propagation works automatically with SQS message attributes. \
**Observability is disabled by default**. When disabled, noop providers are used with minimal performance impact.

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

For more examples including observability setups, see the `/examples` directory:

### Simple Examples
- **`examples/simple/`** - Basic consumer usage with LocalStack
- **`examples/json/`** - JSON message processing with middleware
- **`examples/observability/`** - Complete observability stack with OpenTelemetry

#### Running the Observability Example

```bash
cd examples/observability
docker-compose up -d
```

Access the services:
- **Jaeger UI**: http://localhost:16686 (view distributed traces)
- **Prometheus**: http://localhost:9090 (query metrics)
- **Consumer Metrics**: http://localhost:9464/metrics (Prometheus format)

The example demonstrates:
- ✅ Complete message lifecycle tracing
- ✅ Failed message handling with DLQ
- ✅ Metrics collection for all operations
- ✅ Proper graceful shutdown handling

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License.