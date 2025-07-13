package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/caarlos0/env/v11"
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

// Configuration holds all configurable values
type Configuration struct {
	// AWS/SQS Configuration
	AWSRegion    string `env:"AWS_REGION" envDefault:"us-east-1"`
	AWSEndpoint  string `env:"AWS_ENDPOINT" envDefault:"http://localhost:4566"`
	QueueURL     string `env:"QUEUE_URL" envDefault:"http://localhost:4566/000000000000/test-queue"`
	AWSAccessKey string `env:"AWS_ACCESS_KEY_ID" envDefault:"test"`
	AWSSecretKey string `env:"AWS_SECRET_ACCESS_KEY" envDefault:"test"`

	// OpenTelemetry Configuration
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"http://localhost:4318"`
	MetricsPort  string `env:"METRICS_PORT" envDefault:"9464"`

	// Consumer Configuration
	PollingWorkers    int `env:"POLLING_WORKERS" envDefault:"2"`
	ProcessingWorkers int `env:"PROCESSING_WORKERS" envDefault:"4"`
	MaxMessages       int `env:"MAX_MESSAGES" envDefault:"10"`
	WaitTimeSeconds   int `env:"WAIT_TIME_SECONDS" envDefault:"2"`
	VisibilityTimeout int `env:"VISIBILITY_TIMEOUT" envDefault:"30"`
	ShutdownTimeout   int `env:"SHUTDOWN_TIMEOUT" envDefault:"30"`

	// Debug/Development Configuration
	EnableStdoutTraces bool `env:"ENABLE_STDOUT_TRACES" envDefault:"false"`
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	appConfig, err := loadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Info("Starting SQS Consumer with configuration",
		slog.String("aws_endpoint", appConfig.AWSEndpoint),
		slog.String("queue_url", appConfig.QueueURL),
		slog.String("otel_endpoint", appConfig.OTLPEndpoint),
		slog.String("metrics_port", appConfig.MetricsPort),
	)

	// initialize OpenTelemetry traces and metrics
	traceCleanup, err := initTracing(ctx, logger, appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer traceCleanup()

	metricCleanup, err := initMetrics(ctx, logger, appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}
	defer metricCleanup()

	// create AWS session
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(appConfig.AWSRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(appConfig.AWSAccessKey, appConfig.AWSSecretKey, "")),
		config.WithBaseEndpoint(appConfig.AWSEndpoint),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	sqsClient := sqs.NewFromConfig(cfg)

	// configure observability
	tracerProvider := otel.GetTracerProvider()
	meterProvider := otel.GetMeterProvider()

	logger.Info("Configuring observability",
		slog.String("tracer_provider_type", fmt.Sprintf("%T", tracerProvider)),
		slog.String("meter_provider_type", fmt.Sprintf("%T", meterProvider)),
	)

	obsConfig := observability.NewConfig(
		observability.WithTracerProvider(tracerProvider),
		observability.WithMeterProvider(meterProvider),
		observability.WithPropagator(otel.GetTextMapPropagator()),
		observability.WithServiceName("sqs-consumer-observability-example"),
		observability.WithServiceVersion("1.0.0"),
	)

	// create consumer configuration using NewConfig for best practices
	consumerCfg, err := consumer.NewConfig(appConfig.QueueURL,
		consumer.WithProcessorWorkerPoolSize(int32(appConfig.ProcessingWorkers)),
		consumer.WithPollerWorkerPoolSize(int32(appConfig.PollingWorkers)),
		consumer.WithMaxNumberOfMessages(int32(appConfig.MaxMessages)),
		consumer.WithWaitTimeSeconds(int32(appConfig.WaitTimeSeconds)),
		consumer.WithVisibilityTimeout(int32(appConfig.VisibilityTimeout)),
		consumer.WithErrorNumberThreshold(0),
		consumer.WithGracefulShutdownTimeout(int32(appConfig.ShutdownTimeout)),
		consumer.WithObservability(obsConfig),
	)
	if err != nil {
		log.Fatalf("Failed to create consumer config: %v", err)
	}

	// create consumer with observability (observability middleware is auto-injected)
	sqsConsumer := consumer.NewSQSConsumer(
		*consumerCfg,
		sqsClient,
		consumer.NewJSONMessageAdapter[MyMessage](),
		[]consumer.Middleware[MyMessage]{}, // no manual middleware needed - observability is auto-injected
		logger,
	)

	// define message handler
	handler := consumer.HandlerFunc[MyMessage](func(ctx context.Context, msg MyMessage) error {
		logger.InfoContext(ctx, "Processing message",
			slog.String("id", msg.ID),
			slog.String("type", msg.Type),
			slog.String("content", msg.Content),
			slog.Int("priority", msg.Priority),
		)

		// simulate processing time based on priority
		processingTime := time.Duration(msg.Priority) * 100 * time.Millisecond
		time.Sleep(processingTime)

		// simulate occasional errors for demonstration
		if msg.Priority > 7 {
			return fmt.Errorf("high priority message failed: %s", msg.ID)
		}

		return nil
	})

	shutdownCtx, shutdownCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer shutdownCancel()

	// start message producer in background
	producerCtx, producerCancel := context.WithCancel(ctx)
	go startMessageProducer(producerCtx, sqsClient, appConfig.QueueURL, logger)

	// handle shutdown
	go func() {
		<-shutdownCtx.Done()
		logger.Info("Received shutdown signal, initiating graceful shutdown...")

		// stop the message producer
		producerCancel()

		// use the consumer's Close method for proper shutdown
		if err = sqsConsumer.Close(); err != nil {
			logger.Error("Error during consumer shutdown", slog.Any("error", err))
		}

		logger.Info("Consumer closed successfully")
	}()

	// start consuming messages
	logger.Info("Starting SQS consumer with observability",
		slog.String("queue_url", appConfig.QueueURL),
		slog.Int("polling_workers", int(consumerCfg.PollerWorkerPoolSize)),
		slog.Int("processing_workers", int(consumerCfg.ProcessorWorkerPoolSize)),
	)

	if err = sqsConsumer.Consume(ctx, appConfig.QueueURL, handler); err != nil {
		logger.Error("Consumer failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Consumer shutdown complete")
}

// initTracing initializes OpenTelemetry tracing with multiple exporters
func initTracing(ctx context.Context, logger *slog.Logger, config *Configuration) (func(), error) {
	var cleanupFunctions []func()

	// create resource with service information
	res, err := createResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// initialize trace exporters
	var traceExporters []trace.SpanExporter

	// OTLP trace exporter
	endpoint, insecure, err := parseOTLPEndpoint(config.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OTLP endpoint: %w", err)
	}

	var traceOptions []otlptracehttp.Option
	traceOptions = append(traceOptions, otlptracehttp.WithEndpoint(endpoint))
	if insecure {
		traceOptions = append(traceOptions, otlptracehttp.WithInsecure())
	}
	otlpTraceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	traceExporters = append(traceExporters, otlpTraceExporter)
	cleanupFunctions = append(cleanupFunctions, func() { _ = otlpTraceExporter.Shutdown(ctx) })

	logger.Info("OTLP trace exporter initialized",
		slog.String("endpoint", endpoint),
		slog.Bool("insecure", insecure))

	// stdout trace exporter (only if explicitly enabled for debugging)
	if config.EnableStdoutTraces {
		stdoutTraceExporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
			stdouttrace.WithoutTimestamps(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}

		traceExporters = append(traceExporters, stdoutTraceExporter)
		cleanupFunctions = append(cleanupFunctions, func() { _ = stdoutTraceExporter.Shutdown(ctx) })
		logger.Info("Stdout trace exporter enabled for debugging")
	}

	var traceProviderOptions []trace.TracerProviderOption

	traceProviderOptions = append(traceProviderOptions, trace.WithResource(res))

	for _, exporter := range traceExporters {
		traceProviderOptions = append(traceProviderOptions, trace.WithBatcher(exporter,
			trace.WithBatchTimeout(1*time.Second), // Standard batching timeout
			trace.WithMaxExportBatchSize(512),     // Standard batch size
		))
	}

	traceProvider := trace.NewTracerProvider(traceProviderOptions...)
	otel.SetTracerProvider(traceProvider)

	cleanupFunctions = append(cleanupFunctions, func() { _ = traceProvider.Shutdown(ctx) })

	// set up global text map propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry tracing initialized successfully",
		slog.Int("trace_exporters", len(traceExporters)),
	)

	return func() {
		logger.Info("Shutting down OpenTelemetry tracing...")
		for _, cleanup := range cleanupFunctions {
			cleanup()
		}
	}, nil
}

// initMetrics initializes OpenTelemetry metrics with multiple exporters
func initMetrics(ctx context.Context, logger *slog.Logger, config *Configuration) (func(), error) {
	var cleanupFunctions []func()

	// create resource with service information
	res, err := createResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// initialize metric exporters
	var metricReaders []metric.Reader

	// OTLP metric exporter
	metricEndpoint, metricInsecure, err := parseOTLPEndpoint(config.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OTLP endpoint: %w", err)
	}

	var metricOptions []otlpmetrichttp.Option
	metricOptions = append(metricOptions, otlpmetrichttp.WithEndpoint(metricEndpoint))
	if metricInsecure {
		metricOptions = append(metricOptions, otlpmetrichttp.WithInsecure())
	}
	otlpMetricExporter, err := otlpmetrichttp.New(ctx, metricOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	metricReader := metric.NewPeriodicReader(otlpMetricExporter, metric.WithInterval(10*time.Second))
	metricReaders = append(metricReaders, metricReader)
	cleanupFunctions = append(cleanupFunctions, func() { _ = metricReader.Shutdown(ctx) })

	logger.Info("OTLP metric exporter initialized",
		slog.String("endpoint", metricEndpoint),
		slog.Bool("insecure", metricInsecure))

	// Prometheus metric exporter
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	metricReaders = append(metricReaders, prometheusExporter)

	// start HTTP server for Prometheus metrics
	server := &http.Server{
		Addr:    ":" + config.MetricsPort,
		Handler: nil,
	}

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		logger.Info("Starting Prometheus metrics server", slog.String("port", config.MetricsPort))
		if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Prometheus metrics server failed", slog.Any("error", err))
		}
	}()

	// add server shutdown to clean up functions
	cleanupFunctions = append(cleanupFunctions, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown metrics server", slog.Any("error", err))
		}
	})

	logger.Info("Prometheus metric exporter initialized", slog.String("port", config.MetricsPort))

	// stdout metric exporter (only if explicitly enabled for debugging)
	if config.EnableStdoutTraces {
		stdoutMetricExporter, err := stdoutmetric.New(
			stdoutmetric.WithPrettyPrint(),
			stdoutmetric.WithoutTimestamps(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}

		periodicReader := metric.NewPeriodicReader(stdoutMetricExporter, metric.WithInterval(15*time.Second))
		metricReaders = append(metricReaders, periodicReader)
		cleanupFunctions = append(cleanupFunctions, func() { _ = periodicReader.Shutdown(ctx) })
		logger.Info("Stdout metric exporter enabled for debugging")
	}

	// create metric provider
	meterProviderOptions := []metric.Option{
		metric.WithResource(res),
	}
	for _, reader := range metricReaders {
		meterProviderOptions = append(meterProviderOptions, metric.WithReader(reader))
	}

	meterProvider := metric.NewMeterProvider(meterProviderOptions...)
	otel.SetMeterProvider(meterProvider)

	cleanupFunctions = append(cleanupFunctions, func() { _ = meterProvider.Shutdown(ctx) })

	logger.Info("OpenTelemetry metrics initialized successfully",
		slog.Int("metric_readers", len(metricReaders)),
	)

	return func() {
		logger.Info("Shutting down OpenTelemetry metrics...")
		for _, cleanup := range cleanupFunctions {
			cleanup()
		}
	}, nil
}

// startMessageProducer sends test messages to the SQS queue
func startMessageProducer(ctx context.Context, sqsClient *sqs.Client, queueURL string, logger *slog.Logger) {
	ticker := time.NewTicker(1 * time.Second)
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
				Priority:  (messageID % 10) + 1, // priority 1-10
				Timestamp: time.Now(),
			}

			messageBody, err := json.Marshal(msg)
			if err != nil {
				logger.Error("Failed to marshal message", slog.Any("error", err))
				continue
			}

			// create trace context and inject it into message attributes
			tracer := otel.Tracer("message-producer")

			producerCtx, span := tracer.Start(ctx, "message.produce")

			// inject trace context into message attributes
			carrier := make(map[string]string)
			otel.GetTextMapPropagator().Inject(producerCtx, propagation.MapCarrier(carrier))

			// convert trace context to SQS message attributes
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

// createResource creates a shared OpenTelemetry resource with service information
func createResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("sqs-consumer-observability-example"),
			semconv.ServiceVersion("1.0.0"),
			semconv.ServiceInstanceID("instance-1"),
		),
	)
}

// loadConfiguration loads configuration from environment variables with defaults
func loadConfiguration() (*Configuration, error) {
	cfg := &Configuration{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return cfg, nil
}

// parseOTLPEndpoint parses an OTLP endpoint URL and returns the host:port and insecure flag
func parseOTLPEndpoint(endpoint string) (host string, insecure bool, err error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", false, fmt.Errorf("invalid OTLP endpoint URL: %w", err)
	}

	// Extract host:port
	host = parsedURL.Host
	if host == "" {
		return "", false, fmt.Errorf("OTLP endpoint URL must include host")
	}

	// Add default port if not specified
	if parsedURL.Port() == "" {
		switch parsedURL.Scheme {
		case "https":
			host = host + ":443"
		case "http", "":
			host = host + ":4318" // Default OTLP HTTP port
		default:
			return "", false, fmt.Errorf("unsupported scheme '%s' in OTLP endpoint", parsedURL.Scheme)
		}
	}

	// determine if connection should be insecure based on scheme
	insecure = parsedURL.Scheme == "http" || parsedURL.Scheme == ""

	return host, insecure, nil
}
