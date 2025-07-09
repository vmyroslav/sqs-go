package observability

import "go.opentelemetry.io/otel/attribute"

const (
	meterName = "github.com/vmyroslav/sqs-go/consumer"
)

// MetricName defines a type-safe enumeration for all metric names.
type MetricName string

const (
	// MetricMessages represents a count of messages.
	MetricMessages MetricName = "sqs_messages"
	// MetricPollingRequests represents a count of polling API calls.
	MetricPollingRequests MetricName = "sqs_polling_requests"
	// MetricActiveWorkers represents the number of active workers.
	MetricActiveWorkers MetricName = "sqs_workers_active"
	// MetricBufferSize represents the internal channel buffer size.
	MetricBufferSize MetricName = "sqs_buffer_size"
	// MetricMessagesReceived represents the number of messages received in each polling request.
	MetricMessagesReceived MetricName = "sqs_messages_received"
	// MetricProcessingDuration represents the time to process a message.
	MetricProcessingDuration MetricName = "sqs_processing_duration"
	// MetricPollingDuration represents the time to poll from SQS.
	MetricPollingDuration MetricName = "sqs_polling_duration"
	// MetricAcknowledgmentDuration represents the time to acknowledge a message.
	MetricAcknowledgmentDuration MetricName = "sqs_acknowledgment_duration"
)

// MetricMetadata holds static information about a metric.
type MetricMetadata struct {
	Description string
	Unit        string
}

// metricInfo is a central registry for all metric metadata.
// This makes adding new metrics a single-line change here.
var metricInfo = map[MetricName]MetricMetadata{
	MetricMessages:               {Description: "Total number of SQS messages handled.", Unit: "1"},
	MetricPollingRequests:        {Description: "Total number of SQS polling API requests.", Unit: "1"},
	MetricActiveWorkers:          {Description: "Number of active workers processing messages or polling.", Unit: "1"},
	MetricBufferSize:             {Description: "The configured size of the internal message buffer.", Unit: "1"},
	MetricMessagesReceived:       {Description: "Number of messages received in a single polling request.", Unit: "1"},
	MetricProcessingDuration:     {Description: "The end-to-end duration to process a message.", Unit: "s"},
	MetricPollingDuration:        {Description: "The duration of a single SQS ReceiveMessage API call.", Unit: "s"},
	MetricAcknowledgmentDuration: {Description: "The duration of a message acknowledgment (ack/reject) operation.", Unit: "s"},
}

// MetricOption defines a function that adds a label to a metric.
type MetricOption func() attribute.KeyValue

// WithStatus creates a label for the status of an operation (e.g., "success", "failure").
func WithStatus(status string) MetricOption {
	return func() attribute.KeyValue {
		return attribute.String("status", status)
	}
}

// WithActionMetric creates a metric label for the type of action using type-safe Action constants.
func WithActionMetric(action Action) MetricOption {
	return func() attribute.KeyValue {
		return attribute.String("action", string(action))
	}
}

// WithQueueURLMetric creates a metric label for the SQS queue URL.
func WithQueueURLMetric(queueURL string) MetricOption {
	return func() attribute.KeyValue {
		return attribute.String("queue_url", queueURL)
	}
}

// WithProcessType creates a label for the type of process (e.g., "poller", "processor").
func WithProcessType(processType string) MetricOption {
	return func() attribute.KeyValue {
		return attribute.String("process_type", processType)
	}
}

// WithWorkerID creates a label for the specific worker ID.
func WithWorkerID(workerID string) MetricOption {
	return func() attribute.KeyValue {
		return attribute.String("worker_id", workerID)
	}
}
