[![Action Status](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/vmyroslav/sqs-go/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/vmyroslav/sqs-go/branch/main/graph/badge.svg?token=8F5APGAZT6)](https://codecov.io/gh/vmyroslav/sqs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmyroslav/sqs-consumer)](https://goreportcard.com/report/github.com/vmyroslav/sqs-consumer)
[![Godoc](https://pkg.go.dev/badge/github.com/vmyroslav/home-lib?utm_source=godoc)](https://pkg.go.dev/github.com/vmyroslav/home-lib)

# WIP: Package is under development

# Consumer Package

This package provides a set of tools for consuming messages from an SQS queue. It includes a consumer, message, and middleware components.

## Features

- **Consumer**: The main component that handles the consumption of messages from an SQS queue.
- **Message**: Defines the structure of a message that the consumer can process.
- **Middleware**: Provides a way to add additional functionality to the message processing pipeline, such as time tracking.

## Usage

To use the consumer package, you need to create an instance of the `ConsumerSQS` struct and call its `Consume` method.

```go
consumer := NewConsumerSQS(...)
err := consumer.Consume(ctx, queueURL, messageHandler)
if err != nil {
    log.Fatal(err)
}