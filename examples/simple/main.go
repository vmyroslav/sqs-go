package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/vmyroslav/sqs-go/consumer"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

var (
	queueURL        = " http://sqs.eu-central-1.localhost:4566/000000000000/sqs-test-queue"
	awsBaseEndpoint = "http://localhost:4566"
	timeOut         = 5 * time.Second
)

func main() {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), timeOut)
		logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		handler     = consumer.HandlerFunc[MyMessage](func(ctx context.Context, msg MyMessage) error {
			fmt.Printf("Received message with key %s and body %s\n", msg.Key, msg.Body)

			return nil
		})
		adapter = consumer.MessageAdapterFunc[MyMessage](func(ctx context.Context, msg sqstypes.Message) (MyMessage, error) {
			return MyMessage{Key: *msg.MessageId, Body: *msg.Body}, nil
		})
	)

	defer cancel()

	awsCfg := aws.NewConfig()
	awsCfg.BaseEndpoint = aws.String(awsBaseEndpoint)
	sqsClient := sqs.NewFromConfig(*awsCfg)

	sqsConsumer := consumer.NewSQSConsumer[MyMessage](consumer.Config{
		QueueURL:                queueURL,
		ProcessorWorkerPoolSize: 10,
		PollerWorkerPoolSize:    2,
		MaxNumberOfMessages:     10,
		WaitTimeSeconds:         2,
		VisibilityTimeout:       10,
		ErrorNumberThreshold:    0,
	}, sqsClient, adapter, nil, logger)

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
	for i := 0; i < amount; i++ {
		_, err := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String("Hello, World!"),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

type MyMessage struct {
	Key  string
	Body string
}

func (m MyMessage) Payload() any {
	// TODO implement me
	panic("implement me")
}

func (m MyMessage) Headers() map[string][]byte {
	// TODO implement me
	panic("implement me")
}
