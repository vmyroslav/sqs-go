#!/bin/bash

# Initialize SQS queues for the observability example
echo "Initializing SQS queues..."

# Configure AWS credentials for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# Wait for LocalStack to be ready
echo "Waiting for LocalStack to be ready..."
sleep 5

# Create main queue
echo "Creating main queue..."
aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name test-queue --region us-east-1

# Create dead letter queue
echo "Creating dead letter queue..."
aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name test-queue-dlq --region us-east-1

# Wait a bit for queues to be created
sleep 2

# Get queue URLs with retry
echo "Getting queue URLs..."
MAIN_QUEUE_URL=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-url --queue-name test-queue --region us-east-1 --query 'QueueUrl' --output text 2>/dev/null)
DLQ_URL=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-url --queue-name test-queue-dlq --region us-east-1 --query 'QueueUrl' --output text 2>/dev/null)

# Check if URLs were retrieved successfully
if [ -z "$MAIN_QUEUE_URL" ] || [ "$MAIN_QUEUE_URL" = "None" ]; then
    echo "Failed to get main queue URL"
    exit 1
fi

if [ -z "$DLQ_URL" ] || [ "$DLQ_URL" = "None" ]; then
    echo "Failed to get DLQ URL"
    exit 1
fi

# Get DLQ ARN
echo "Getting DLQ ARN..."
DLQ_ARN=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names QueueArn --region us-east-1 --query 'Attributes.QueueArn' --output text 2>/dev/null)

if [ -z "$DLQ_ARN" ] || [ "$DLQ_ARN" = "None" ]; then
    echo "Failed to get DLQ ARN"
    exit 1
fi

# Configure redrive policy for main queue
echo "Configuring redrive policy..."
aws --endpoint-url=http://localhost:4566 sqs set-queue-attributes \
  --queue-url "$MAIN_QUEUE_URL" \
  --attributes '{
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"'$DLQ_ARN'\",\"maxReceiveCount\":3}",
    "VisibilityTimeoutSeconds": "30"
  }' \
  --region us-east-1

echo "SQS queues initialized successfully!"
echo "Main queue URL: $MAIN_QUEUE_URL"
echo "Dead letter queue URL: $DLQ_URL"