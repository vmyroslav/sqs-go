package consumer

import (
	"context"
	"errors"
	"testing"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
)

func TestNewCompositeAcknowledger(t *testing.T) {
	mockAck := newMockAcknowledger(nil, 0)
	mockRejecter := newMockAcknowledger(nil, 0)

	ca := newCompositeAcknowledger(mockAck, mockRejecter)

	assert.NotNil(t, ca)
	assert.Equal(t, mockAck, ca.Acknowledger)
	assert.Equal(t, mockRejecter, ca.Rejecter)
}

func TestCompositeAcknowledger_Ack(t *testing.T) {
	tests := []struct {
		name        string
		ackError    error
		expectError bool
	}{
		{
			name:        "successful acknowledgment",
			ackError:    nil,
			expectError: false,
		},
		{
			name:        "acknowledgment fails",
			ackError:    errors.New("ack failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAck := newMockAcknowledger(tt.ackError, 1)
			mockRejecter := newMockAcknowledger(nil, 0)
			ca := newCompositeAcknowledger(mockAck, mockRejecter)
			ctx := context.Background()
			msg := sqstypes.Message{}

			err := ca.Ack(ctx, msg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompositeAcknowledger_Reject(t *testing.T) {
	tests := []struct {
		name        string
		rejectError error
		expectError bool
	}{
		{
			name:        "successful rejection",
			rejectError: nil,
			expectError: false,
		},
		{
			name:        "rejection fails",
			rejectError: errors.New("reject failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAck := newMockAcknowledger(nil, 0)
			mockRejecter := newMockAcknowledger(tt.rejectError, 1)

			ca := newCompositeAcknowledger(mockAck, mockRejecter)
			ctx := context.Background()
			msg := sqstypes.Message{}

			err := ca.Reject(ctx, msg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
