package consumer

type AcknowledgmentStrategy string

const (
	SyncAcknowledgment        AcknowledgmentStrategy = "sync"
	ImmediateAcknowledgment   AcknowledgmentStrategy = "immediate"
	ExponentialAcknowledgment AcknowledgmentStrategy = "exponential"
)

func createAcknowledger(
	strategy AcknowledgmentStrategy,
	queueURL string,
	sqsClient sqsConnector,
) acknowledger {
	switch strategy {
	case ImmediateAcknowledgment:
		return newImmediateAcknowledger(queueURL, sqsClient)
	case ExponentialAcknowledgment:
		return newExponentialAcknowledger(queueURL, sqsClient)
	case SyncAcknowledgment:
		fallthrough
	default:
		return newSyncAcknowledger(queueURL, sqsClient)
	}
}
