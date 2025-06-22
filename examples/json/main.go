package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/vmyroslav/sqs-go/consumer"
)

var (
	queueURL        = " http://sqs.eu-central-1.localhost:4566/000000000000/sqs-test-queue"
	awsBaseEndpoint = "http://localhost:4566"
	timeOut         = 5 * time.Second

	sqsConfig = consumer.Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: 10,
		PollerWorkerPoolSize:    2,
		MaxNumberOfMessages:     10,
		WaitTimeSeconds:         2,
		VisibilityTimeout:       10,
		ErrorNumberThreshold:    0,
	}
)

func main() {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), timeOut)
		logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		handler     = consumer.HandlerFunc[MyMessage](func(_ context.Context, msg MyMessage) error {
			fmt.Printf("Received message with key %s and body %s\n", msg.Key, msg.Body)

			return nil
		})
		adapter     = consumer.NewJSONMessageAdapter[MyMessage]()
		middlewares []consumer.Middleware[MyMessage]
	)

	middlewares = append(
		middlewares,
		newTimeTrackingMiddleware[MyMessage](),
		consumer.MiddlewareAdapter[MyMessage](newLoggingMiddleware()),
	)

	defer cancel()

	awsCfg := aws.NewConfig()
	awsCfg.BaseEndpoint = aws.String(awsBaseEndpoint)
	sqsClient := sqs.NewFromConfig(*awsCfg)

	sqsConsumer := consumer.NewSQSConsumer[MyMessage](sqsConfig, sqsClient, adapter, middlewares, logger)

	if err := produceMessages(sqsClient, 10); err != nil {
		panic(fmt.Errorf("failed to produce message: %w", err))
	}

	go func() {
		if err := sqsConsumer.Consume(ctx, queueURL, handler); err != nil {
			fmt.Println(err)
		}
	}()

	// notify context to stop the sqsConsumer by the signal
	signal.NotifyContext(ctx, os.Interrupt)
	<-ctx.Done()
}

func produceMessages(sqsClient *sqs.Client, amount int) error {
	m := MyMessage{
		Key:  "key",
		Body: "body",
	}

	str, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal MyMessage: %w", err)
	}

	for i := 0; i < amount; i++ {
		_, err := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String(string(str)),
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	return nil
}

type MyMessage struct {
	Key  string `json:"key"`
	Body string `json:"body"`
}

func newLoggingMiddleware() consumer.Middleware[any] {
	return func(next consumer.HandlerFunc[any]) consumer.HandlerFunc[any] {
		return func(ctx context.Context, msg any) error {
			fmt.Printf("Generic message received: %v\n", msg)
			return next.Handle(ctx, msg)
		}
	}
}

func newTimeTrackingMiddleware[T any]() consumer.Middleware[T] {
	return func(next consumer.HandlerFunc[T]) consumer.HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			start := time.Now()

			err := next.Handle(ctx, msg)

			elapsed := time.Since(start)
			fmt.Printf("Message processed in %s\n", elapsed)

			return fmt.Errorf("handler error: %w", err)
		}
	}
}
