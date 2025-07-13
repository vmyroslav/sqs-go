#!/bin/bash

# Create SQS queue
awslocal sqs create-queue --queue-name sqs-test-queue

echo "SQS queue 'sqs-test-queue' created successfully"