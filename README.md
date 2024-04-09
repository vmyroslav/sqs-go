[![Action Status](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/vmyroslav/sqs-go/branch/main/graph/badge.svg?token=8F5APGAZT6)](https://codecov.io/gh/vmyroslav/sqs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmyroslav/sqs-consumer)](https://goreportcard.com/report/github.com/vmyroslav/sqs-consumer)
[![Godoc](https://pkg.go.dev/badge/github.com/vmyroslav/home-lib?utm_source=godoc)](https://pkg.go.dev/github.com/vmyroslav/sqs-go)

# SQS Consumer Library for Go

## Introduction

The SQS Consumer library simplifies the consumption of messages from AWS SQS (Simple Queue Service). 
By abstracting the boilerplate code required to poll, process, and acknowledge SQS messages, this library allows developers to focus on writing business logic specific to their application's requirements.

## Features

- Easy setup and integration with AWS SQS.
- Generic message processing through custom handlers.
- Support for JSON message format out of the box.
- Middleware support for extending library functionality.
- Customizable worker pools for polling and message processing.
- Graceful shutdown handling.
  
## Installation

To install the SQS Consumer library, use the go get command:
```bash
 go get github.com/vmyroslav/sqs-go
```

## Usage

### Basic Usage
To use the library, create an instance of SQSConsumer with the desired configuration and message handler.
Below is a simplified example:

```go
package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/yourusername/sqs-consumer/consumer"
)

func main() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("unable to load SDK config")
	}
	
	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := "your-sqs-queue-url"
	
	adapter := consumer.NewJSONMessageAdapter[YourMessageType]()
	processorCfg := consumer.NewDefaultConfig(queueURL)
	sqsConsumer := consumer.NewSQSConsumer[YourMessageType](processorCfg, sqsClient, adapter, nil, nil)
	
	handler := consumer.HandlerFunc[YourMessageType](func(ctx context.Context, msg YourMessageType) error {
		// Process the message here
		return nil
	})
	
	if err := sqsConsumer.Consume(ctx, queueURL, handler); err != nil {
		panic(err)
	}
}
```

Replace `YourMessageType` with your message struct.

## Configuration
The `Config` struct allows customization of the consumer behavior:

- `QueueURL`: URL of the SQS queue.
- `ProcessorWorkerPoolSize`: Number of workers processing messages.
- `PollerWorkerPoolSize`: Number of workers polling messages from SQS.
- Additional configuration options are available for message polling and visibility handling.

## Middleware
Middleware can be used to extend functionality, such as error handling or logging, etc.
Implement the `Middleware` interface and pass the middleware to `NewSQSConsumer`.

## Examples

The `/examples` directory contains example applications demonstrating various library features.

# Consumer Package

This package provides a set of tools for consuming messages from an SQS queue. It includes a consumer, message, and middleware components.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request.