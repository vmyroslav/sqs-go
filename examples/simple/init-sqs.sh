#!/bin/bash

# Create SQS queue for simple example
awslocal sqs create-queue --queue-name sqs-test-queue

echo "SQS queue 'sqs-test-queue' created successfully"