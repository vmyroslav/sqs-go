[![Actions Status](https://github.com/vmyroslav/<project>/actions/workflows/deployment.yaml/badge.svg)](https://github.com/vmyroslav/<project>/actions)

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