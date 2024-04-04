package consumer

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/mock"
)

func TestSyncAcknowledger_Ack(t *testing.T) {
	sqsClient := newMockSqsConnector(t)
	ack := newSyncAcknowledger("testQueue", sqsClient)

	msg := sqstypes.Message{
		MessageId:     aws.String("1"),
		ReceiptHandle: aws.String("handle"),
	}

	sqsClient.On(
		"DeleteMessage",
		mock.Anything,
		mock.Anything,
	).Return(&sqs.DeleteMessageOutput{}, nil)

	err := ack.Ack(context.Background(), msg)
	require.NoError(t, err)
	sqsClient.AssertExpectations(t)
}
