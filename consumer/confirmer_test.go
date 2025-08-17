package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
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
			sqsClient := newMocksqsConnector(t)
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
	sqsClient := newMocksqsConnector(t)
	ack := newSyncAcknowledger("testQueue", sqsClient)

	msg := sqstypes.Message{
		MessageId:     aws.String("1"),
		ReceiptHandle: aws.String("handle"),
	}

	err := ack.Reject(context.Background(), msg)

	require.NoError(t, err)
	sqsClient.AssertExpectations(t)
}

func TestImmediateRejector_Ack(t *testing.T) {
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
			sqsClient := newMocksqsConnector(t)
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

func TestImmediateRejector_newVisibilityTimeoutInput(t *testing.T) {
	a := newImmediateRejector("http://localhost:4566/000000000000/queue", nil)

	assert.NotPanics(t, func() {
		id := "bdgsbsdbg"
		receipt := &id
		cmvi := a.newVisibilityTimeoutInput(receipt)
		assert.NotNil(t, cmvi)
		assert.NotNil(t, cmvi.ReceiptHandle)
		assert.Equal(t, id, *cmvi.ReceiptHandle)
		assert.NotNil(t, cmvi.VisibilityTimeout)
		assert.Zero(t, cmvi.VisibilityTimeout)
		assert.NotNil(t, cmvi.QueueUrl)
		assert.Equal(t, "http://localhost:4566/000000000000/queue", *cmvi.QueueUrl)
	})
}

func TestImmediateAcknowledger_Reject(t *testing.T) {
	tests := []struct {
		name                    string
		changeMessageVisibility func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
		wantErr                 bool
	}{
		{
			name: "ChangeMessageVisibility success",
			changeMessageVisibility: func(_ context.Context, _ *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
				return &sqs.ChangeMessageVisibilityOutput{}, nil
			},
			wantErr: false,
		},
		{
			name: "ChangeMessageVisibility error",
			changeMessageVisibility: func(_ context.Context, _ *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
				return nil, errors.New("change message visibility error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqsClient := newMocksqsConnector(t)
			ack := newImmediateRejector("testQueue", sqsClient)

			msg := sqstypes.Message{
				MessageId:     aws.String("1"),
				ReceiptHandle: aws.String("handle"),
			}

			sqsClient.On(
				"ChangeMessageVisibility",
				mock.Anything,
				mock.Anything,
			).Return(tt.changeMessageVisibility)

			err := ack.Reject(context.Background(), msg)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			sqsClient.AssertExpectations(t)
		})
	}
}

func TestExponentialRejector_Reject(t *testing.T) {
	tests := []struct {
		name                    string
		messageAttributes       map[string]string
		changeMessageVisibility func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
		wantErr                 bool
	}{
		{
			name:              "ChangeMessageVisibility success - first retry",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "1"},
			changeMessageVisibility: func(_ context.Context, _ *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
				return &sqs.ChangeMessageVisibilityOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:              "ChangeMessageVisibility success - second retry",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "2"},
			changeMessageVisibility: func(_ context.Context, _ *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
				return &sqs.ChangeMessageVisibilityOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:              "ChangeMessageVisibility error",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "1"},
			changeMessageVisibility: func(_ context.Context, _ *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
				return nil, errors.New("change message visibility error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqsClient := newMocksqsConnector(t)
			ack := newExponentialRejector("testQueue", sqsClient)

			msg := sqstypes.Message{
				MessageId:     aws.String("1"),
				ReceiptHandle: aws.String("handle"),
				Attributes:    tt.messageAttributes,
			}

			sqsClient.On(
				"ChangeMessageVisibility",
				mock.Anything,
				mock.Anything,
			).Return(tt.changeMessageVisibility)

			err := ack.Reject(context.Background(), msg)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			sqsClient.AssertExpectations(t)
		})
	}
}

func TestExponentialRejector_calculateVisibilityTimeout(t *testing.T) {
	ack := newExponentialRejector("testQueue", nil)

	tests := []struct {
		name              string
		messageAttributes map[string]string
		expectedTimeout   int32
	}{
		{
			name:              "first retry",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "1"},
			expectedTimeout:   0, // 100ms base delay
		},
		{
			name:              "second retry",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "2"},
			expectedTimeout:   0, // 200ms
		},
		{
			name:              "third retry",
			messageAttributes: map[string]string{"ApproximateReceiveCount": "3"},
			expectedTimeout:   0, // 400ms
		},
		{
			name:              "no receive count attribute",
			messageAttributes: map[string]string{},
			expectedTimeout:   0, // defaults to 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := sqstypes.Message{
				Attributes: tt.messageAttributes,
			}

			timeout := ack.calculateVisibilityTimeout(msg)
			assert.GreaterOrEqual(t, timeout, tt.expectedTimeout)
		})
	}
}

func TestExponentialRejector_Ack(t *testing.T) {
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
			sqsClient := newMocksqsConnector(t)
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
