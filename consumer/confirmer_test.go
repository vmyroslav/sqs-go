package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncAcknowledger_Ack(t *testing.T) {
	tests := []struct {
		name          string
		deleteMessage func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
		wantErr       bool
	}{
		{
			name: "DeleteMessage success",
			deleteMessage: func(_ context.Context, _ *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
				return &sqs.DeleteMessageOutput{}, nil
			},
			wantErr: false,
		},
		{
			name: "DeleteMessage error",
			deleteMessage: func(_ context.Context, _ *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
				return nil, errors.New("delete message error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			).Return(tt.deleteMessage)

			err := ack.Ack(context.Background(), msg)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			sqsClient.AssertExpectations(t)
		})
	}
}

func TestSyncAcknowledger_Reject(t *testing.T) {
	sqsClient := newMockSqsConnector(t)
	ack := newSyncAcknowledger("testQueue", sqsClient)

	msg := sqstypes.Message{
		MessageId:     aws.String("1"),
		ReceiptHandle: aws.String("handle"),
	}

	err := ack.Reject(context.Background(), msg)

	require.NoError(t, err)
	sqsClient.AssertExpectations(t)
}
