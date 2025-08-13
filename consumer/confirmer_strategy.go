package consumer

type AcknowledgmentStrategy string

const (
	// SyncAcknowledgment is default acknowledge strategy. It doesn't reject message.
	SyncAcknowledgment AcknowledgmentStrategy = "sync"
	// ImmediateRejector changes VisibilityTimeout of the message to 0 after unsuccessful processing.
	ImmediateRejector AcknowledgmentStrategy = "immediate"
	// ExponentialRejector exponentially changing VisibilityTimeout of the message after unsuccessful processing.
	ExponentialRejector AcknowledgmentStrategy = "exponential"
)

func createAcknowledger(
	strategy AcknowledgmentStrategy,
	queueURL string,
	sqsClient sqsConnector,
) acknowledger {
	switch strategy {
	case ImmediateRejector:
		return newImmediateRejector(queueURL, sqsClient)
	case ExponentialRejector:
		return newExponentialRejector(queueURL, sqsClient)
	case SyncAcknowledgment:
		fallthrough //nolint:gocritic
	default:
		return newSyncAcknowledger(queueURL, sqsClient)
	}
}
