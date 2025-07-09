package observability

import (
	"context"
	"sync"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/propagation"
)

// carrierPool provides a pool of reusable sqsMessageCarrier instances to eliminate per-message allocations
var carrierPool = sync.Pool{
	New: func() interface{} {
		return &sqsMessageCarrier{}
	},
}

// ExtractTraceContext extracts distributed trace context from SQS message attributes.
//
// Behavior:
// - If producer set trace context: Returns context with distributed trace information
// - If producer did NOT set trace context: Returns the original context unchanged
//
// When used with tracer.Span(), this ensures:
// - WITH producer context → Creates child span (distributed tracing)
// - WITHOUT producer context → Creates root span (new independent trace)
func ExtractTraceContext(ctx context.Context, msg sqstypes.Message, p propagation.TextMapPropagator) context.Context {
	carrier := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
	defer carrierPool.Put(carrier)

	carrier.Reset(msg.MessageAttributes)

	return p.Extract(ctx, carrier)
}

// sqsMessageCarrier implements propagation.TextMapCarrier for SQS message attributes
type sqsMessageCarrier struct {
	attributes map[string]sqstypes.MessageAttributeValue
}

var _ propagation.TextMapCarrier = (*sqsMessageCarrier)(nil)

// Reset resets the carrier with new message attributes
// This allows the carrier to be reused from the pool
func (c *sqsMessageCarrier) Reset(attributes map[string]sqstypes.MessageAttributeValue) {
	c.attributes = attributes
}

func (c *sqsMessageCarrier) Get(key string) string {
	if attr, exists := c.attributes[key]; exists && attr.StringValue != nil {
		return *attr.StringValue
	}

	return ""
}

func (c *sqsMessageCarrier) Set(key, value string) {
	// we don't need in the consumer, but implemented for completeness
	if c.attributes == nil {
		c.attributes = make(map[string]sqstypes.MessageAttributeValue)
	}

	c.attributes[key] = sqstypes.MessageAttributeValue{
		DataType:    toPtr("String"),
		StringValue: &value,
	}
}

func (c *sqsMessageCarrier) Keys() []string {
	if len(c.attributes) == 0 {
		return nil
	}

	keys := make([]string, 0, len(c.attributes))
	for key := range c.attributes {
		keys = append(keys, key)
	}

	return keys
}

// toPtr returns a pointer to the given value of any type.
func toPtr[T any](v T) *T {
	return &v
}
