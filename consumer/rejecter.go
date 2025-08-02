package consumer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/cenkalti/backoff/v5"
)

type sqsRejecter struct {
	sqsClient sqsConnector
	QueueURL  string
	Strategy  RejectStrategy
}

func newSQSRejecter(queueURL string, strategy RejectStrategy, sqsClient sqsConnector) *sqsRejecter {
	return &sqsRejecter{
		sqsClient: sqsClient,
		QueueURL:  queueURL,
		Strategy:  strategy,
	}
}

type ChangeVisibilityError struct {
	Err error
	Msg sqstypes.Message
}

func (c *ChangeVisibilityError) Error() string {
	return fmt.Sprintf("change visibility for msg: %s: %s", aws.ToString(c.Msg.ReceiptHandle), c.Err)
}

func (c *ChangeVisibilityError) Unwrap() error {
	return c.Err
}

// Ack is nop in SQSRejecter
func (s *sqsRejecter) Ack(_ context.Context, _ sqstypes.Message) error { return nil }

func (s *sqsRejecter) Reject(ctx context.Context, msg sqstypes.Message) error {
	_, err := s.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(s.QueueURL),
		ReceiptHandle:     msg.ReceiptHandle,
		VisibilityTimeout: s.Strategy.VisibilityTimeout(msg),
	})
	if err != nil {
		return &ChangeVisibilityError{Msg: msg, Err: err}
	}

	return nil
}

type ImmediateRejectStrategy struct{}

func NewImmediateRejectStrategy() *ImmediateRejectStrategy {
	return &ImmediateRejectStrategy{}
}

func (i *ImmediateRejectStrategy) VisibilityTimeout(_ sqstypes.Message) int32 { return 0 }

type ExponentialBackoffRejectStrategy struct {
	MaxRetries int
}

func NewExponentialBackoffRejectStrategy(maxRetries int) *ExponentialBackoffRejectStrategy {
	return &ExponentialBackoffRejectStrategy{MaxRetries: maxRetries}
}

const sqsMaxVisibilityTimeout = 15 * time.Minute

func (b *ExponentialBackoffRejectStrategy) VisibilityTimeout(msg sqstypes.Message) int32 {
	if b.MaxRetries <= 0 {
		return 0
	}

	ebo := backoff.NewExponentialBackOff()
	ebo.MaxInterval = sqsMaxVisibilityTimeout

	retryCount := int32(0)

	if msg.Attributes != nil {
		if receiveCount, exists := msg.Attributes["ApproximateReceiveCount"]; exists {
			if count, err := strconv.Atoi(receiveCount); err == nil {
				retryCount = int32(count) - 1 //nolint:gosec // G109 Possible overflow on count variable
			}
		}
	}

	if int(retryCount) >= b.MaxRetries {
		return 0
	}

	for range retryCount {
		ebo.NextBackOff()
	}

	duration := ebo.NextBackOff()
	if duration == backoff.Stop {
		return 0
	}

	return int32(duration.Seconds())
}
