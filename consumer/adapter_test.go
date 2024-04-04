package consumer

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
)

func TestJsonMessageAdapterTransform(t *testing.T) {
	t.Parallel()

	type myMessage struct {
		Key    string `json:"key"`
		Body   string `json:"body"`
		Nested struct {
			Number int `json:"number"`
		} `json:"nested"`
	}

	var (
		adapter = NewJSONMessageAdapter[myMessage]()
		ctx     = context.Background()
	)

	tests := []struct {
		name        string
		msg         sqstypes.Message
		expectedMsg myMessage
		expectError bool
	}{
		{
			name: "should transform valid json message",
			msg: sqstypes.Message{
				Body: aws.String(`{"key":"value","body":"body","nested":{"number":1}}`),
			},
			expectedMsg: myMessage{
				Key:  "value",
				Body: "body",
				Nested: struct {
					Number int `json:"number"`
				}{Number: 1},
			},
			expectError: false,
		},
		{
			name: "should return error for invalid json message",
			msg: sqstypes.Message{
				Body: aws.String(`invalid json`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := adapter.Transform(ctx, tt.msg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMsg, result)
			}
		})
	}
}

func TestDummyAdapterTransform(t *testing.T) {
	t.Parallel()

	adapter := NewDummyAdapter[sqstypes.Message]()

	tests := []struct {
		name        string
		msg         sqstypes.Message
		expectedMsg sqstypes.Message
	}{
		{
			name: "should return the same message",
			msg: sqstypes.Message{
				MessageId:     aws.String("1"),
				ReceiptHandle: aws.String("handle"),
				Body:          aws.String("body"),
			},
			expectedMsg: sqstypes.Message{
				MessageId:     aws.String("1"),
				ReceiptHandle: aws.String("handle"),
				Body:          aws.String("body"),
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := adapter.Transform(ctx, tt.msg)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMsg, result)
		})
	}
}
