package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer"
)

var (
	queueURL        = "http://localhost:4566/000000000000/sqs-test-queue"
	awsBaseEndpoint = "http://localhost:4566"
)

func main() {
	var (
		ctx     = context.Background()
		logger  = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		handler = consumer.HandlerFunc[MyMessage](func(_ context.Context, msg MyMessage) error {
			fmt.Printf("Received message with key %s and body %s\n", msg.Key, msg.Body)

			return nil
		})
		adapter = consumer.MessageAdapterFunc[MyMessage](func(_ context.Context, msg sqstypes.Message) (MyMessage, error) {
			return MyMessage{Key: *msg.MessageId, Body: *msg.Body}, nil
		})
	)

	awsCfg := aws.NewConfig()
	awsCfg.BaseEndpoint = aws.String(awsBaseEndpoint)
	sqsClient := sqs.NewFromConfig(*awsCfg)

	consumerConfig, err := consumer.NewConfig(queueURL,
		consumer.WithProcessorWorkerPoolSize(10),
		consumer.WithPollerWorkerPoolSize(2),
		consumer.WithMaxNumberOfMessages(10),
		consumer.WithWaitTimeSeconds(2),
		consumer.WithVisibilityTimeout(10),
		consumer.WithErrorNumberThreshold(0),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create consumer config: %w", err))
	}

	sqsConsumer := consumer.NewSQSConsumer[MyMessage](*consumerConfig, sqsClient, adapter, nil, logger)

	if err = produceMessages(sqsClient, 10); err != nil {
		panic(fmt.Errorf("failed to produce message: %w", err))
	}

	// setup signal handling for graceful shutdown
	shutdownCtx, shutdownCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer shutdownCancel()

	// handle shutdown
	go func() {
		<-shutdownCtx.Done()
		logger.Info("Received shutdown signal, initiating graceful shutdown...")

		// use the consumer's Close method for proper shutdown
		if err = sqsConsumer.Close(); err != nil {
			logger.Error("Error during consumer shutdown", "error", err)
		}

		logger.Info("Consumer shutdown completed successfully")
	}()

	// start consuming messages (blocks until shutdown)
	if err = sqsConsumer.Consume(ctx, queueURL, handler); err != nil {
		fmt.Printf("Consumer failed: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Consumer shutdown complete")
}

func produceMessages(sqsClient *sqs.Client, amount int) error {
	for i := 0; i < amount; i++ {
		_, err := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String("Hello, World!"),
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	return nil
}

type MyMessage struct {
	Key  string
	Body string
}
