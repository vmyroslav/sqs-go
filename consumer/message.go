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
