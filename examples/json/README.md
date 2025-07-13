# JSON Consumer Example

Simple example demonstrating SQS message consumption with JSON message adapter and middleware.

## Features

- JSON message parsing with type safety
- Custom middleware (time tracking, logging)
- Graceful shutdown with signal handling

## Quick Start

1. **Start LocalStack SQS**:
   ```bash
   docker-compose up -d
   ```

2. **Run the consumer**:
   ```bash
   go run main.go
   ```

The example will:
- Start LocalStack with SQS service
- Create a test queue automatically
- Produce 10 test messages
- Consume and process messages with middleware
- Demonstrate graceful shutdown on SIGINT/SIGTERM

## Configuration

- **Queue URL**: `http://localhost:4566/000000000000/sqs-test-queue`
- **AWS Endpoint**: `http://localhost:4566` (LocalStack)
- **Workers**: 10 processing, 2 polling
- **Message Format**: JSON with `key` and `body` fields

## Cleanup

```bash
docker-compose down
```