package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmyroslav/sqs-go/consumer"
	"github.com/vmyroslav/sqs-go/consumer/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// MyMessage represents a JSON message structure
type MyMessage struct {
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Priority  int       `json:"priority"`
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Initialize OpenTelemetry
	cleanup, err := initOpenTelemetry(ctx, logger)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer cleanup()

	// Create AWS session for LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		config.WithBaseEndpoint("http://localhost:4566"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := "http://localhost:4566/000000000000/test-queue"

	// Configure observability with multiple exporters
	obsConfig := observability.NewConfig(
		observability.WithTracerProvider(otel.GetTracerProvider()),
		observability.WithMeterProvider(otel.GetMeterProvider()),
		observability.WithServiceName("observability-example"),
		observability.WithServiceVersion("1.0.0"),
	)

	// Create consumer configuration
	consumerCfg := &consumer.Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: 4,
		PollerWorkerPoolSize:    2,
		MaxNumberOfMessages:     10,
		WaitTimeSeconds:         2,
		VisibilityTimeout:       30,
		ErrorNumberThreshold:    0,
		GracefulShutdownTimeout: 30,
		Observability:           obsConfig,
	}

	// Create consumer with observability (observability middleware is auto-injected)
	sqsConsumer := consumer.NewSQSConsumer(
		*consumerCfg,
		sqsClient,
		consumer.NewJSONMessageAdapter[MyMessage](),
		[]consumer.Middleware[MyMessage]{}, // No manual middleware needed - observability is auto-injected
		logger,
	)

	// Start message producer in background
	go startMessageProducer(ctx, sqsClient, queueURL, logger)

	// Define message handler
	handler := consumer.HandlerFunc[MyMessage](func(ctx context.Context, msg MyMessage) error {
		logger.InfoContext(ctx, "Processing message",
			slog.String("id", msg.ID),
			slog.String("type", msg.Type),
			slog.String("content", msg.Content),
			slog.Int("priority", msg.Priority),
		)

		// Simulate processing time based on priority
		processingTime := time.Duration(msg.Priority) * 100 * time.Millisecond
		time.Sleep(processingTime)

		// Simulate occasional errors for demonstration
		if msg.Priority > 7 {
			return fmt.Errorf("high priority message failed: %s", msg.ID)
		}

		return nil
	})

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Received shutdown signal, stopping consumer...")
		cancel()
	}()

	// Start consuming messages
	logger.Info("Starting SQS consumer with observability",
		slog.String("queue_url", queueURL),
		slog.Int("polling_workers", int(consumerCfg.PollerWorkerPoolSize)),
		slog.Int("processing_workers", int(consumerCfg.ProcessorWorkerPoolSize)),
	)

	if err := sqsConsumer.Consume(ctx, queueURL, handler); err != nil {
		logger.Error("Consumer failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Consumer shutdown complete")
}

// initOpenTelemetry initializes OpenTelemetry with multiple exporters
func initOpenTelemetry(ctx context.Context, logger *slog.Logger) (func(), error) {
	var cleanupFunctions []func()

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("sqs-consumer-observability-example"),
			semconv.ServiceVersion("1.0.0"),
			semconv.ServiceInstanceID("instance-1"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize trace exporters
	traceExporters := []trace.SpanExporter{}

	// OTLP trace exporter (try to connect to local collector)
	otlpTraceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("http://localhost:4318"),
		otlptracehttp.WithInsecure())
	if err != nil {
		logger.Warn("Failed to create OTLP trace exporter", slog.Any("error", err))
	} else {
		traceExporters = append(traceExporters, otlpTraceExporter)
		cleanupFunctions = append(cleanupFunctions, func() { _ = otlpTraceExporter.Shutdown(ctx) })

		logger.Info("OTLP trace exporter initialized")
	}

	// Stdout trace exporter (for local development)
	stdoutTraceExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithoutTimestamps(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
	}

	traceExporters = append(traceExporters, stdoutTraceExporter)
	cleanupFunctions = append(cleanupFunctions, func() { _ = stdoutTraceExporter.Shutdown(ctx) })

	// Create trace provider with batch processor
	var traceProviderOptions []trace.TracerProviderOption

	traceProviderOptions = append(traceProviderOptions, trace.WithResource(res))

	for _, exporter := range traceExporters {
		traceProviderOptions = append(traceProviderOptions, trace.WithBatcher(exporter))
	}

	traceProvider := trace.NewTracerProvider(traceProviderOptions...)
	otel.SetTracerProvider(traceProvider)

	cleanupFunctions = append(cleanupFunctions, func() { _ = traceProvider.Shutdown(ctx) })

	// Initialize metric exporters
	metricReaders := []metric.Reader{}

	// OTLP metric exporter (try to connect to local collector)
	otlpMetricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint("http://localhost:4318"),
		otlpmetrichttp.WithInsecure())
	if err != nil {
		logger.Warn("Failed to create OTLP metric exporter", slog.Any("error", err))
	} else {
		metricReader := metric.NewPeriodicReader(otlpMetricExporter, metric.WithInterval(10*time.Second))
		metricReaders = append(metricReaders, metricReader)
		cleanupFunctions = append(cleanupFunctions, func() { _ = metricReader.Shutdown(ctx) })

		logger.Info("OTLP metric exporter initialized")
	}

	// Prometheus metric exporter (expose metrics on :9464)
	prometheusExporter, err := prometheus.New()
	if err != nil {
		logger.Warn("Failed to create Prometheus exporter", slog.Any("error", err))
	} else {
		metricReaders = append(metricReaders, prometheusExporter)
		// Start HTTP server for Prometheus metrics
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			logger.Info("Starting Prometheus metrics server on :9464")

			if err := http.ListenAndServe(":9464", nil); err != nil {
				logger.Error("Prometheus metrics server failed", slog.Any("error", err))
			}
		}()

		logger.Info("Prometheus metric exporter initialized on :9464")
	}

	// Stdout metric exporter (for local development)
	stdoutMetricExporter, err := stdoutmetric.New(
		stdoutmetric.WithPrettyPrint(),
		stdoutmetric.WithoutTimestamps(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
	}

	metricReader := metric.NewPeriodicReader(stdoutMetricExporter, metric.WithInterval(15*time.Second))
	metricReaders = append(metricReaders, metricReader)
	cleanupFunctions = append(cleanupFunctions, func() { _ = metricReader.Shutdown(ctx) })

	// Create metric provider
	meterProviderOptions := []metric.Option{
		metric.WithResource(res),
	}
	for _, reader := range metricReaders {
		meterProviderOptions = append(meterProviderOptions, metric.WithReader(reader))
	}

	meterProvider := metric.NewMeterProvider(meterProviderOptions...)
	otel.SetMeterProvider(meterProvider)

	cleanupFunctions = append(cleanupFunctions, func() { _ = meterProvider.Shutdown(ctx) })

	// Set up global text map propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry initialized successfully",
		slog.Int("trace_exporters", len(traceExporters)),
		slog.Int("metric_readers", len(metricReaders)),
	)

	return func() {
		logger.Info("Shutting down OpenTelemetry...")

		for _, cleanup := range cleanupFunctions {
			cleanup()
		}
	}, nil
}

// startMessageProducer sends test messages to the SQS queue
func startMessageProducer(ctx context.Context, sqsClient *sqs.Client, queueURL string, logger *slog.Logger) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	messageID := 1

	for {
		select {
		case <-ctx.Done():
			logger.Info("Message producer stopping")
			return
		case <-ticker.C:
			msg := MyMessage{
				ID:        fmt.Sprintf("msg-%d", messageID),
				Type:      "test",
				Content:   fmt.Sprintf("Test message content %d", messageID),
				Priority:  (messageID % 10) + 1, // Priority 1-10
				Timestamp: time.Now(),
			}

			messageBody, err := json.Marshal(msg)
			if err != nil {
				logger.Error("Failed to marshal message", slog.Any("error", err))
				continue
			}

			// Create trace context and inject it into message attributes
			tracer := otel.Tracer("message-producer")

			producerCtx, span := tracer.Start(ctx, "message.produce")

			// Inject trace context into message attributes
			carrier := make(map[string]string)
			otel.GetTextMapPropagator().Inject(producerCtx, propagation.MapCarrier(carrier))

			// Convert trace context to SQS message attributes
			messageAttributes := make(map[string]types.MessageAttributeValue)
			for k, v := range carrier {
				messageAttributes[k] = types.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String(v),
				}
			}

			_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
				QueueUrl:          aws.String(queueURL),
				MessageBody:       aws.String(string(messageBody)),
				MessageAttributes: messageAttributes,
			})
			if err != nil {
				logger.Error("Failed to send message", slog.Any("error", err))
				span.RecordError(err)
				span.End()

				continue
			}

			logger.Info("Message sent", slog.String("message_id", msg.ID), slog.Int("priority", msg.Priority))

			messageID++
			span.End()
		}
	}
}
