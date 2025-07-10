# SQS Consumer Observability Example

This example demonstrates comprehensive observability features of the SQS consumer library using OpenTelemetry. It shows how to set up distributed tracing, metrics collection, and integration with popular observability tools.

## Features Demonstrated

### 🔍 Distributed Tracing
- **Jaeger Integration**: Complete distributed tracing with span hierarchy
- **Context Propagation**: Automatic trace context extraction from SQS message attributes
- **Span Lifecycle**: Covers polling, processing, transformation, and acknowledgment operations
- **Error Tracking**: Automatic error recording and span status management

### 📊 Comprehensive Metrics
- **Message Processing**: Count and duration of message processing
- **Polling Operations**: SQS polling frequency and latency
- **Acknowledgment Tracking**: Success/failure rates for message acknowledgments
- **Worker Monitoring**: Active worker counts and buffer sizes
- **Error Rates**: Success/error ratios across all operations

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
         │              ┌─────────────────┐    ┌─────────────────┐
         │              │   Prometheus    │ -> │     Grafana     │
         │              │   (Metrics)     │    │ (Visualization) │
         │              └─────────────────┘    └─────────────────┘
         │
         v
┌─────────────────┐
│   LocalStack    │
│   (SQS Mock)    │
└─────────────────┘
```

## Prerequisites

- Docker and Docker Compose
- Go 1.21 or higher
- AWS CLI (for LocalStack interaction)

## Quick Start

1. **Start the observability stack**:
   ```bash
   cd examples/observability
   docker-compose up -d
   ```

2. **Wait for services to be ready** (approximately 30 seconds):
   ```bash
   # Check service health
   docker-compose ps
   ```

3. **Run the example**:
   ```bash
   go run main.go
   ```

4. **Access the observability tools**:
   - **Jaeger UI**: http://localhost:16686 (Distributed tracing)
   - **Prometheus**: http://localhost:9090 (Metrics)
   - **Grafana**: http://localhost:3000 (Dashboards - admin/admin)

## Environment Configuration

The example supports multiple exporters via environment variables:

```bash
# Enable OTLP exporter
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Enable Jaeger exporter
export JAEGER_ENDPOINT=http://localhost:14268/api/traces

# Enable Prometheus exporter
export PROMETHEUS_ENABLED=true
```

## What You'll See

### Distributed Tracing (Jaeger)
- **Span Hierarchy**: Complete trace from message receipt to acknowledgment
- **Service Map**: Visual representation of service dependencies
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
rate(sqs_messages_total[5m])

# Average processing time
histogram_quantile(0.95, rate(sqs_processing_duration_seconds_bucket[5m]))

# Error rate
rate(sqs_messages_total{status="error"}[5m]) / rate(sqs_messages_total[5m])
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

## Advanced Configuration

### Custom Resource Attributes
```go
res, err := resource.New(ctx,
    resource.WithAttributes(
        semconv.ServiceName("my-sqs-consumer"),
        semconv.ServiceVersion("2.0.0"),
        semconv.ServiceInstanceID("prod-instance-1"),
        semconv.DeploymentEnvironment("production"),
    ),
)
```

### Custom Sampling
```go
traceProvider := trace.NewTracerProvider(
    trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
    trace.WithResource(res),
)
```

### Metric Customization
```go
// Custom metric collection interval
metricReader := metric.NewPeriodicReader(
    stdoutMetricExporter,
    metric.WithInterval(5*time.Second),
)
```

## Troubleshooting

### Common Issues

1. **Services not starting**: Check Docker logs with `docker-compose logs [service]`
2. **No traces appearing**: Verify OTLP endpoints and trace context propagation
3. **Missing metrics**: Check Prometheus scrape targets at http://localhost:9090/targets
4. **SQS connection errors**: Ensure LocalStack is running and healthy

### Debug Mode
Enable debug logging:
```bash
export OTEL_LOG_LEVEL=debug
```

## Production Considerations

- **Resource Usage**: Monitor memory usage with multiple exporters
- **Sampling**: Implement appropriate trace sampling for high-volume scenarios
- **Security**: Configure proper authentication for production observability backends
- **Retention**: Set appropriate data retention policies
- **Alerting**: Create alerts based on error rates and performance metrics

## Further Reading

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/instrumentation/go/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [SQS Consumer Library Architecture](../../README.md#architecture)