# SQS Consumer Observability Example

This example demonstrates comprehensive observability features of the SQS consumer library using OpenTelemetry. It shows how to set up distributed tracing, metrics collection, and integration with popular observability tools.

## Features Demonstrated

### 🔍 Distributed Tracing
- **Jaeger Integration**: Complete distributed tracing with span hierarchy
- **Context Propagation**: Automatic trace context extraction from SQS message attributes
- **Span Lifecycle**: Covers processing, transformation, and acknowledgment operations
- **Error Tracking**: Automatic error recording and span status management

### 📊 Metrics
- **Message Processing**: Count and duration of message processing
- **Polling Operations**: SQS polling frequency and latency
- **Acknowledgment Tracking**: Success/failure rates for message acknowledgments
- **Worker Monitoring**: Active worker counts and buffer sizes

### 🛠️ Multiple Exporters
- **OTLP**: Industry-standard OpenTelemetry Protocol for traces and metrics
- **Jaeger**: Distributed tracing UI and analysis
- **Prometheus**: Time-series metrics collection
- **Stdout**: Local development debugging

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   SQS Consumer  │ -> │ OTLP Collector  │ -> │     Jaeger      │
│   (Your App)    │    │  (Aggregation)  │    │   (Tracing)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │
         │                        v
         │              ┌─────────────────┐
         │              │   Prometheus    │
         │              │   (Metrics)     │
         │              └─────────────────┘
         │
         v
┌─────────────────┐
│   LocalStack    │
│   (SQS Mock)    │
└─────────────────┘
```

## Configuration

This example is fully configurable via environment variables, making it easy to run both locally and in Docker.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_REGION` | `us-east-1` | AWS region for SQS |
| `AWS_ENDPOINT` | `http://localhost:4566` | AWS endpoint (LocalStack) |
| `QUEUE_URL` | `http://localhost:4566/000000000000/test-queue` | Full SQS queue URL |
| `AWS_ACCESS_KEY_ID` | `test` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | `test` | AWS secret key |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OpenTelemetry collector endpoint |
| `METRICS_PORT` | `9464` | Port for Prometheus metrics endpoint |
| `POLLING_WORKERS` | `2` | Number of polling worker goroutines |
| `PROCESSING_WORKERS` | `4` | Number of processing worker goroutines |
| `MAX_MESSAGES` | `10` | Maximum messages per SQS batch |
| `WAIT_TIME_SECONDS` | `2` | SQS long polling wait time |
| `VISIBILITY_TIMEOUT` | `30` | SQS message visibility timeout |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout in seconds |
| `ENABLE_STDOUT_TRACES` | `false` | Enable verbose trace/metric logs to stdout (debugging) |

### Configuration Files

- **`docker-compose.yml`**: Pre-configured for container networking

## Prerequisites

- Docker and Docker Compose
- Go 1.24 or higher

## Quick Start

### Option 1: Fully Containerized (Recommended)

1. **Run the complete example with Docker**:
   ```bash
   docker-compose up --build
   ```

2. **Access the observability tools**:
   - **Jaeger UI**: http://localhost:16686 (Distributed tracing)
   - **Prometheus**: http://localhost:9090 (Metrics)
   - **SQS Consumer Metrics**: http://localhost:9464/metrics (Direct Prometheus endpoint)

## What You'll See

### Distributed Tracing (Jaeger)
- **Span Hierarchy**: Complete trace from message receipt to acknowledgment
- **Error Tracking**: Failed message processing with detailed error information
- **Performance Analysis**: Latency distribution and bottleneck identification

### Metrics (Prometheus)
Available metrics include:
- `sqs_messages_total`: Total messages processed (with status labels)
- `sqs_processing_duration_seconds`: Message processing time histogram
- `sqs_polling_duration_seconds`: SQS polling latency
- `sqs_acknowledgment_duration_seconds`: Acknowledgment operation time
- `sqs_workers_active`: Current active worker count
- `sqs_buffer_size`: Internal message buffer size
- `sqs_polling_requests_total`: Total polling requests made

### Example Queries
```promql
# Message processing rate
rate(sqs_messages_ratio_total[5m])
```

## Message Flow

1. **Producer**: Sends messages with trace context to SQS
2. **Poller**: Retrieves messages and creates polling spans
3. **Processor**: Processes messages with full observability
4. **Middleware**: Adds additional instrumentation
5. **Acknowledger**: Confirms or rejects messages

## Testing Error Scenarios

The example includes built-in error scenarios:
- Messages with priority > 7 will fail processing
- Failed messages demonstrate error tracing and metrics
- Dead letter queue handling (after 3 retries)

## Cleanup

```bash
docker-compose down -v
```