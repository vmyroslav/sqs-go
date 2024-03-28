package consumer

import (
	"time"
)

const (
	HeaderKeyMessageID = "MessageId"
	HeaderKeyTimestamp = "Timestamp"
	HeaderKeyRetries   = "Retries"
)

type DefaultMessage struct {
	headers map[string][]byte
	key     any
	payload any

	Timestamp time.Time

	queueUrl string
}

func (d DefaultMessage) Key() any {
	// TODO implement me
	panic("implement me")
}

func (d DefaultMessage) Payload() any {
	// TODO implement me
	panic("implement me")
}

func (d DefaultMessage) Headers() map[string][]byte {
	// TODO implement me
	panic("implement me")
}
