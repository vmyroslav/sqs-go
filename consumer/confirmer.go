package consumer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// syncAcknowledger is used to ack/reject messages.
// An example of very simple sync implementation is provided without any error handling on our side.
type syncAcknowledger struct {
	sqsClient sqsConnector
	queueURL  string
}

func newSyncAcknowledger(queueURL string, sqsClient sqsConnector) *syncAcknowledger {
	return &syncAcknowledger{sqsClient: sqsClient, queueURL: queueURL}
}

func (a *syncAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &a.queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (a *syncAcknowledger) Reject(_ context.Context, _ sqstypes.Message) error { return nil }
