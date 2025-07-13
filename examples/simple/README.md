# Simple Consumer Example

Minimal example demonstrating basic SQS message consumption with custom message adapter.

## Features

- Custom message adapter (MessageId → Key, Body → Body)
- No middleware (pure message handling)
- Graceful shutdown with signal handling
- LocalStack integration for local development

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
- Produce 10 "Hello, World!" messages
- Consume and print each message
- Demonstrate graceful shutdown on SIGINT/SIGTERM

## Configuration

- **Queue URL**: `http://localhost:4566/000000000000/sqs-test-queue`
- **AWS Endpoint**: `http://localhost:4566` (LocalStack)
- **Workers**: 10 processing, 2 polling
- **Message Format**: Plain text converted to struct with Key/Body

## Cleanup

```bash
docker-compose down
```