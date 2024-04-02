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
		adapter = NewJsonMessageAdapter[myMessage]()
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
	type myMessage struct {
		Key    string `json:"key"`
		Body   string `json:"body"`
		Nested struct {
			Number int `json:"number"`
		} `json:"nested"`
	}

	var (
		adapter = NewDummyAdapter[myMessage]()
		ctx     = context.Background()
	)

	tests := []struct {
		name        string
		msg         sqstypes.Message
		expectedMsg myMessage
		expectError bool
	}{
		{
			name: "should always return empty message",
			msg: sqstypes.Message{
				Body: aws.String(`{"key":"value","body":"body","nested":{"number":1}}`),
			},
			expectedMsg: myMessage{},
			expectError: false,
		},
		{
			name: "should return empty message for sqstypes.Message",
			msg: sqstypes.Message{
				Body: aws.String(`{"MessageId":"123","ReceiptHandle":"abc","MD5OfBody":"def","Body":"ghi","Attributes":null,"MD5OfMessageAttributes":null,"MessageAttributes":null}`),
			},
			expectedMsg: myMessage{},
			expectError: false,
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
