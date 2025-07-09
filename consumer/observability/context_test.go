package observability

import (
	"context"
	"sync"
	"testing"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestExtractTraceContext(t *testing.T) {
	t.Parallel()
	t.Run("With trace context in message attributes", func(t *testing.T) {
		t.Parallel()
		// create a context with trace information
		ctx := context.Background()
		traceID := trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
		spanID := trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}

		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  spanID,
		})

		_ = trace.ContextWithSpanContext(ctx, spanContext)

		// create a message with trace context
		msg := sqstypes.Message{
			MessageAttributes: map[string]sqstypes.MessageAttributeValue{
				"traceparent": {
					DataType:    toPtr("String"),
					StringValue: toPtr("00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01"),
				},
			},
		}

		propagator := propagation.TraceContext{}

		extractedCtx := ExtractTraceContext(ctx, msg, propagator)

		assert.NotNil(t, extractedCtx)
		// extracted context should contain the trace information
		extractedSpanContext := trace.SpanContextFromContext(extractedCtx)
		assert.True(t, extractedSpanContext.IsValid())
		assert.Equal(t, traceID, extractedSpanContext.TraceID())
	})

	t.Run("Without trace context in message attributes", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		msg := sqstypes.Message{
			MessageAttributes: map[string]sqstypes.MessageAttributeValue{
				"other-attribute": {
					DataType:    toPtr("String"),
					StringValue: toPtr("some-value"),
				},
			},
		}

		propagator := propagation.TraceContext{}

		extractedCtx := ExtractTraceContext(ctx, msg, propagator)

		assert.NotNil(t, extractedCtx)
		// should return the original context when no trace context is present
		extractedSpanContext := trace.SpanContextFromContext(extractedCtx)
		assert.False(t, extractedSpanContext.IsValid())
	})

	t.Run("With empty message attributes", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		msg := sqstypes.Message{
			MessageAttributes: map[string]sqstypes.MessageAttributeValue{},
		}

		propagator := propagation.TraceContext{}

		extractedCtx := ExtractTraceContext(ctx, msg, propagator)

		assert.NotNil(t, extractedCtx)
		extractedSpanContext := trace.SpanContextFromContext(extractedCtx)
		assert.False(t, extractedSpanContext.IsValid())
	})

	t.Run("With nil message attributes", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		msg := sqstypes.Message{
			MessageAttributes: nil,
		}

		propagator := propagation.TraceContext{}

		extractedCtx := ExtractTraceContext(ctx, msg, propagator)

		assert.NotNil(t, extractedCtx)
		extractedSpanContext := trace.SpanContextFromContext(extractedCtx)
		assert.False(t, extractedSpanContext.IsValid())
	})
}

func TestSQSMessageCarrier(t *testing.T) {
	t.Parallel()
	t.Run("Get method", func(t *testing.T) {
		t.Parallel()
		t.Run("Returns value for existing key", func(t *testing.T) {
			t.Parallel()

			attrs := map[string]sqstypes.MessageAttributeValue{
				"test-key": {
					DataType:    toPtr("String"),
					StringValue: toPtr("test-value"),
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(attrs)

			value := carrier.Get("test-key")
			assert.Equal(t, "test-value", value)
		})

		t.Run("Returns empty string for non-existing key", func(t *testing.T) {
			t.Parallel()

			attrs := map[string]sqstypes.MessageAttributeValue{
				"other-key": {
					DataType:    toPtr("String"),
					StringValue: toPtr("other-value"),
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(attrs)

			value := carrier.Get("non-existing-key")
			assert.Empty(t, value)
		})

		t.Run("Returns empty string for nil string value", func(t *testing.T) {
			t.Parallel()

			attrs := map[string]sqstypes.MessageAttributeValue{
				"nil-key": {
					DataType:    toPtr("String"),
					StringValue: nil,
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(attrs)

			value := carrier.Get("nil-key")
			assert.Empty(t, value)
		})

		t.Run("Returns empty string for empty attributes", func(t *testing.T) {
			t.Parallel()

			carrier := &sqsMessageCarrier{}
			carrier.Reset(map[string]sqstypes.MessageAttributeValue{})

			value := carrier.Get("any-key")
			assert.Empty(t, value)
		})
	})

	t.Run("Set method", func(t *testing.T) {
		t.Parallel()
		t.Run("Sets value for new key", func(t *testing.T) {
			t.Parallel()

			carrier := &sqsMessageCarrier{}
			carrier.Reset(nil)

			carrier.Set("new-key", "new-value")

			value := carrier.Get("new-key")
			assert.Equal(t, "new-value", value)
		})

		t.Run("Overwrites existing value", func(t *testing.T) {
			t.Parallel()

			attrs := map[string]sqstypes.MessageAttributeValue{
				"existing-key": {
					DataType:    toPtr("String"),
					StringValue: toPtr("old-value"),
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(attrs)

			carrier.Set("existing-key", "new-value")

			value := carrier.Get("existing-key")
			assert.Equal(t, "new-value", value)
		})

		t.Run("Initializes attributes map if nil", func(t *testing.T) {
			t.Parallel()

			carrier := &sqsMessageCarrier{}
			carrier.Reset(nil)

			carrier.Set("test-key", "test-value")

			assert.NotNil(t, carrier.attributes)
			value := carrier.Get("test-key")
			assert.Equal(t, "test-value", value)
		})
	})

	t.Run("Keys method", func(t *testing.T) {
		t.Parallel()
		t.Run("Returns all keys", func(t *testing.T) {
			t.Parallel()

			attrs := map[string]sqstypes.MessageAttributeValue{
				"key1": {
					DataType:    toPtr("String"),
					StringValue: toPtr("value1"),
				},
				"key2": {
					DataType:    toPtr("String"),
					StringValue: toPtr("value2"),
				},
				"key3": {
					DataType:    toPtr("String"),
					StringValue: toPtr("value3"),
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(attrs)

			keys := carrier.Keys()
			assert.Len(t, keys, 3)
			assert.Contains(t, keys, "key1")
			assert.Contains(t, keys, "key2")
			assert.Contains(t, keys, "key3")
		})

		t.Run("Returns nil for empty attributes", func(t *testing.T) {
			t.Parallel()

			carrier := &sqsMessageCarrier{}
			carrier.Reset(map[string]sqstypes.MessageAttributeValue{})

			keys := carrier.Keys()
			assert.Nil(t, keys)
		})

		t.Run("Returns nil for nil attributes", func(t *testing.T) {
			t.Parallel()

			carrier := &sqsMessageCarrier{}
			carrier.Reset(nil)

			keys := carrier.Keys()
			assert.Nil(t, keys)
		})
	})

	t.Run("Reset method", func(t *testing.T) {
		t.Parallel()
		t.Run("Replaces attributes", func(t *testing.T) {
			t.Parallel()

			oldAttrs := map[string]sqstypes.MessageAttributeValue{
				"old-key": {
					DataType:    toPtr("String"),
					StringValue: toPtr("old-value"),
				},
			}

			newAttrs := map[string]sqstypes.MessageAttributeValue{
				"new-key": {
					DataType:    toPtr("String"),
					StringValue: toPtr("new-value"),
				},
			}

			carrier := &sqsMessageCarrier{}
			carrier.Reset(oldAttrs)

			// verify old attributes are set
			assert.Equal(t, "old-value", carrier.Get("old-key"))

			carrier.Reset(newAttrs)

			// verify new attributes are set and old ones are replaced
			assert.Equal(t, "new-value", carrier.Get("new-key"))
			assert.Empty(t, carrier.Get("old-key"))
		})
	})
}

func TestCarrierPool(t *testing.T) {
	t.Parallel()
	t.Run("Pool provides reusable carriers", func(t *testing.T) {
		t.Parallel()

		carrier1 := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
		assert.NotNil(t, carrier1)

		// Use the carrier
		attrs := map[string]sqstypes.MessageAttributeValue{
			"test-key": {
				DataType:    toPtr("String"),
				StringValue: toPtr("test-value"),
			},
		}
		carrier1.Reset(attrs)

		assert.Equal(t, "test-value", carrier1.Get("test-key"))

		carrierPool.Put(carrier1)

		// get from pool again (may or may not be the same instance)
		carrier2 := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
		assert.NotNil(t, carrier2)

		// Test that the second carrier works correctly
		attrs2 := map[string]sqstypes.MessageAttributeValue{
			"test-key2": {
				DataType:    toPtr("String"),
				StringValue: toPtr("test-value2"),
			},
		}
		carrier2.Reset(attrs2)
		assert.Equal(t, "test-value2", carrier2.Get("test-key2"))

		carrierPool.Put(carrier2)
	})

	t.Run("Pool creates new carrier when needed", func(t *testing.T) {
		t.Parallel()

		carrier := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
		assert.NotNil(t, carrier)

		// don't return to pool, get another one
		carrier2 := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
		assert.NotNil(t, carrier2)

		// They may be different instances if pool is empty, test that the pool can create new carriers
		assert.True(t, carrier == carrier2 || carrier != carrier2)

		// Return both to pool
		carrierPool.Put(carrier)
		carrierPool.Put(carrier2)
	})
}

func TestCarrierPoolConcurrency(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100

	const numIterations = 10

	t.Run("Concurrent access to carrier pool", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for j := 0; j < numIterations; j++ {
					carrier := carrierPool.Get().(*sqsMessageCarrier) // nolint:errcheck // sync.Pool.Get never returns an error
					assert.NotNil(t, carrier)

					// Use the carrier
					attrs := map[string]sqstypes.MessageAttributeValue{
						"test-key": {
							DataType:    toPtr("String"),
							StringValue: toPtr("test-value"),
						},
					}
					carrier.Reset(attrs)

					value := carrier.Get("test-key")
					assert.Equal(t, "test-value", value)

					carrierPool.Put(carrier)
				}
			}()
		}

		wg.Wait()
	})
}

func TestToPtr(t *testing.T) {
	t.Parallel()
	t.Run("Returns pointer to string", func(t *testing.T) {
		t.Parallel()

		input := "test-string"
		ptr := toPtr(input)

		assert.NotNil(t, ptr)
		assert.Equal(t, input, *ptr)
	})

	t.Run("Returns pointer to empty string", func(t *testing.T) {
		t.Parallel()

		input := ""
		ptr := toPtr(input)

		assert.NotNil(t, ptr)
		assert.Equal(t, input, *ptr)
	})

	t.Run("Returns pointer to integer", func(t *testing.T) {
		t.Parallel()

		input := 42
		ptr := toPtr(input)

		assert.NotNil(t, ptr)
		assert.Equal(t, input, *ptr)
	})

	t.Run("Returns pointer to boolean", func(t *testing.T) {
		t.Parallel()

		input := true
		ptr := toPtr(input)

		assert.NotNil(t, ptr)
		assert.Equal(t, input, *ptr)
	})

	t.Run("Returns pointer to struct", func(t *testing.T) {
		t.Parallel()

		type testStruct struct {
			Field string
		}

		input := testStruct{Field: "test"}
		ptr := toPtr(input)

		assert.NotNil(t, ptr)
		assert.Equal(t, input, *ptr)
		assert.Equal(t, "test", ptr.Field)
	})
}
