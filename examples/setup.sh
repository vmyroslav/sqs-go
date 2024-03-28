#!/usr/bin/env bash
     set -euo pipefail

     create_queue() {
         local QUEUE_NAME_TO_CREATE=$1
         awslocal --endpoint-url=http://"${LOCALSTACK_HOST}":4566 --region="${DEFAULT_REGION}" sqs create-queue --queue-name "${QUEUE_NAME_TO_CREATE}"
     }

     create_queues() {
       local QUEUE_NAME_TO_CREATE=$1
       local DLQ_NAME_TO_CREATE=$2
       # Create the DLQ
       DLQ_URL=$(awslocal --endpoint-url=http://"${LOCALSTACK_HOST}":4566 --region="${DEFAULT_REGION}" sqs create-queue --queue-name "${DLQ_NAME_TO_CREATE}" --query 'QueueUrl' --output text)

       # Get the ARN of the DLQ
       DLQ_ARN=$(awslocal sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

       REDRIVE_POLICY='{\"deadLetterTargetArn\":\"'${DLQ_ARN}'\",\"maxReceiveCount\":\"3\"}'

       # Create the main SQS queue with a Redrive Policy
       awslocal --endpoint-url=http://"${LOCALSTACK_HOST}":4566 --region="${DEFAULT_REGION}" sqs create-queue --queue-name "${QUEUE_NAME_TO_CREATE}" --attributes "{\"RedrivePolicy\": \"${REDRIVE_POLICY}\"}"
     }


     get_queue_url() {
         local QUEUE_URL=$(awslocal --endpoint-url=http://"${LOCALSTACK_HOST}":4566 --region="${DEFAULT_REGION}" sqs list-queues --query 'QueueUrls[?ends_with(@, `'$QUEUE_NAME'`)]' --output text)

         # Check if the queue URL is empty
         if [ -z "$QUEUE_URL" ]; then
             echo "Queue not found"
             return 1
         fi

         echo "$QUEUE_URL"
     }

     purge_queue() {
         local QUEUE_URL=$1
         awslocal --endpoint-url=http://"${LOCALSTACK_HOST}":4566 --region="${DEFAULT_REGION}" sqs purge-queue --queue-url "$QUEUE_URL"
     }

     DLQ_NAME="${QUEUE_NAME}_dlq"
     echo "creating queue $QUEUE_NAME and DLQ $DLQ_NAME"
     create_queues "${QUEUE_NAME}" "${DLQ_NAME}"

     QUEUE_URL=$(get_queue_url)
     export QUEUE_URL
     echo "Queue URL is set to $QUEUE_URL"
     DLQ_QUEUE_URL=$(get_queue_url)
     echo "DLQ queue URL is set to $DLQ_QUEUE_URL"
