package consumer

import (
	"context"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type CompositeAcknowledger struct {
	Rejecter     acknowledger
	Acknowledger acknowledger
}

func newCompositeAcknowledger(acknowledger, rejecter acknowledger) *CompositeAcknowledger {
	return &CompositeAcknowledger{
		Rejecter:     rejecter,
		Acknowledger: acknowledger,
	}
}

func (c *CompositeAcknowledger) Ack(ctx context.Context, msg sqstypes.Message) error {
	return c.Acknowledger.Ack(ctx, msg) //nolint:wrapcheck // force wrapping error
}

func (c *CompositeAcknowledger) Reject(ctx context.Context, msg sqstypes.Message) error {
	return c.Rejecter.Reject(ctx, msg) //nolint:wrapcheck // force wrapping error
}
