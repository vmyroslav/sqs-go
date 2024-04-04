package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// JsonMessageAdapter is a message adapter for json messages
type JsonMessageAdapter[T any] struct{}

func NewJsonMessageAdapter[T any]() *JsonMessageAdapter[T] {
	return &JsonMessageAdapter[T]{}
}

func (a *JsonMessageAdapter[T]) Transform(_ context.Context, msg sqstypes.Message) (T, error) {
	var m T

	if err := json.Unmarshal([]byte(*msg.Body), &m); err != nil {
		return m, fmt.Errorf("failed to unmarshal message body: %w", err)
	}

	return m, nil
}

type DummyAdapter[T sqstypes.Message] struct{}

func NewDummyAdapter[T sqstypes.Message]() *DummyAdapter[T] {
	return &DummyAdapter[T]{}
}

func (a *DummyAdapter[T]) Transform(_ context.Context, msg sqstypes.Message) (T, error) {
	return T(msg), nil
}
