package consumer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSRejecter struct {
	sqsClient sqsConnector
	QueueURL  string
	Strategy  RejectStrategy
}

func newSQSRejecter(queueURL string, strategy RejectStrategy, sqsClient sqsConnector) *SQSRejecter {
	return &SQSRejecter{
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
func (s *SQSRejecter) Ack(_ context.Context, _ sqstypes.Message) error { return nil }

func (s *SQSRejecter) Reject(ctx context.Context, msg sqstypes.Message) error {
	_, err := s.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(s.QueueURL),
		ReceiptHandle:     msg.ReceiptHandle,
		VisibilityTimeout: s.VisibilityTimeout(), // immediately available
	})
	if err != nil {
		return &ChangeVisibilityError{Msg: msg, Err: err}
	}

	return nil
}

func (s *SQSRejecter) VisibilityTimeout() int32 {
	switch s.Strategy {
	case Immediate:
		return 0
	}

	panic("unreachable")
}
