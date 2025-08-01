package consumer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSRejecter struct {
	sqsClient         *sqs.Client // TODO: change to [sqsConnector]
	QueueURL          string
	VisibilityTimeout int
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
		VisibilityTimeout: 0, // immediately available
	})
	if err != nil {
		return &ChangeVisibilityError{Msg: msg, Err: err}
	}

	return nil
}
