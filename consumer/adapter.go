package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// JsonSQSMessageAdapter is a message adapter for json messages
type JsonSQSMessageAdapter[T Message] struct{}

func NewJsonSQSMessageAdapter[T Message]() *JsonSQSMessageAdapter[T] {
	return &JsonSQSMessageAdapter[T]{}
}

func (a *JsonSQSMessageAdapter[T]) Transform(_ context.Context, msg sqstypes.Message) (T, error) {
	var m T

	if err := json.Unmarshal([]byte(*msg.Body), &m); err != nil {
		return m, fmt.Errorf("failed to unmarshal message body: %w", err)
	}

	return m, nil
}

type NullSQSMessageAdapter[T Message] struct{}

func NewNullSQSMessageAdapter[T Message]() *NullSQSMessageAdapter[T] {
	return &NullSQSMessageAdapter[T]{}
}
