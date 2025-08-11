package consumer

type AcknowledgmentStrategy string

const (
	// SyncAcknowledgment is default acknowledgement strategy. It doesn't reject message.
	SyncAcknowledgment AcknowledgmentStrategy = "sync"
	// ImmediateAcknowledgment changes VisibilityTimeout of the message to 0 after unsuccessful processing.
	ImmediateAcknowledgment AcknowledgmentStrategy = "immediate"
	// ExponentialAcknowledgment exponentially changing VisibilityTimeout of the message after unsuccessful processing.
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
		fallthrough //nolint:gocritic
	default:
		return newSyncAcknowledger(queueURL, sqsClient)
	}
}
