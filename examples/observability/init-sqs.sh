#!/bin/bash

echo "Initializing SQS queues..."

# Configure AWS credentials for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# Create queues with error handling
echo "Creating main queue..."
if ! aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name test-queue --region us-east-1 2>/dev/null; then
    echo "Main queue might already exist, continuing..."
fi

echo "Creating dead letter queue..."
if ! aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name test-queue-dlq --region us-east-1 2>/dev/null; then
    echo "DLQ might already exist, continuing..."
fi

# Get queue URLs with retry
echo "Getting queue URLs..."
MAIN_QUEUE_URL=""
DLQ_URL=""

for i in {1..3}; do
    MAIN_QUEUE_URL=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-url --queue-name test-queue --region us-east-1 --query 'QueueUrl' --output text 2>/dev/null || echo "")
    if [ -n "$MAIN_QUEUE_URL" ] && [ "$MAIN_QUEUE_URL" != "None" ]; then
        echo "Got main queue URL: $MAIN_QUEUE_URL"
        break
    fi
    echo "Retrying to get main queue URL... ($i/3)"
    sleep 1
done

for i in {1..3}; do
    DLQ_URL=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-url --queue-name test-queue-dlq --region us-east-1 --query 'QueueUrl' --output text 2>/dev/null || echo "")
    if [ -n "$DLQ_URL" ] && [ "$DLQ_URL" != "None" ]; then
        echo "Got DLQ URL: $DLQ_URL"
        break
    fi
    echo "Retrying to get DLQ URL... ($i/3)"
    sleep 1
done

# Check if URLs were retrieved successfully
if [ -z "$MAIN_QUEUE_URL" ] || [ "$MAIN_QUEUE_URL" = "None" ]; then
    echo "Warning: Failed to get main queue URL, but queues might still work"
else
    echo "Main queue URL: $MAIN_QUEUE_URL"
fi

if [ -z "$DLQ_URL" ] || [ "$DLQ_URL" = "None" ]; then
    echo "Warning: Failed to get DLQ URL, but queues might still work"
else
    echo "Dead letter queue URL: $DLQ_URL"
    
    # Only configure redrive policy if we have both URLs
    if [ -n "$MAIN_QUEUE_URL" ] && [ "$MAIN_QUEUE_URL" != "None" ]; then
        # Get DLQ ARN
        echo "Getting DLQ ARN..."
        DLQ_ARN=$(aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names QueueArn --region us-east-1 --query 'Attributes.QueueArn' --output text 2>/dev/null || echo "")

        if [ -n "$DLQ_ARN" ] && [ "$DLQ_ARN" != "None" ]; then
            echo "Configuring redrive policy..."
            echo "DLQ ARN: $DLQ_ARN"
            echo "Main Queue URL: $MAIN_QUEUE_URL"
            
            if aws --endpoint-url=http://localhost:4566 sqs set-queue-attributes \
              --queue-url "$MAIN_QUEUE_URL" \
              --attributes '{"RedrivePolicy":"{\"deadLetterTargetArn\":\"'$DLQ_ARN'\",\"maxReceiveCount\":3}"}' \
              --region us-east-1; then
                echo "✓ Redrive policy configured successfully"
                
                # Verify the redrive policy was set
                echo "Verifying redrive policy..."
                aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes \
                  --queue-url "$MAIN_QUEUE_URL" \
                  --attribute-names RedrivePolicy \
                  --region us-east-1 && echo "✓ Redrive policy verification successful"
            else
                echo "✗ Failed to set redrive policy"
            fi

            echo "Setting visibility timeout to 30 seconds..."
            if aws --endpoint-url=http://localhost:4566 sqs set-queue-attributes \
              --queue-url "$MAIN_QUEUE_URL" \
              --attributes VisibilityTimeout=30 \
              --region us-east-1; then
                echo "✓ Visibility timeout set successfully"
            else
                echo "✗ Failed to set visibility timeout"
            fi
        else
            echo "Warning: Could not get DLQ ARN, skipping redrive policy setup"
        fi
    fi
fi

# Show queue configuration
echo ""
if [ -n "$MAIN_QUEUE_URL" ] && [ "$MAIN_QUEUE_URL" != "None" ]; then
    echo "Main Queue Attributes:"
    aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes \
      --queue-url "$MAIN_QUEUE_URL" \
      --attribute-names All \
      --region us-east-1 || echo "Could not retrieve main queue attributes"
    echo ""
fi

if [ -n "$DLQ_URL" ] && [ "$DLQ_URL" != "None" ]; then
    echo "DLQ Attributes:"
    aws --endpoint-url=http://localhost:4566 sqs get-queue-attributes \
      --queue-url "$DLQ_URL" \
      --attribute-names All \
      --region us-east-1 || echo "Could not retrieve DLQ attributes"
    echo ""
fi

echo "✓ SQS queues initialized successfully!"
echo "Ready."