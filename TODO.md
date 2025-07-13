# SQS-GO - To-Do List

## Implement Consumer Group for Multi-Queue Management

To simplify the management of multiple SQS consumers within a single application, this feature will introduce a `ConsumerGroup` orchestrator. This will abstract away the boilerplate concurrency logic (e.g., `sync.WaitGroup`, error channels) required to run and shut down several consumers, providing a clean, centralized API.

- [ ] Create a new `ConsumerGroup` struct to hold a collection of consumer instances.
- [ ] Implement a `NewGroup()` constructor and an `AddConsumer()` method for a declarative setup.
- [ ] Develop a `Run(ctx context.Context)` method, likely using `errgroup`, to start all consumers and manage their lifecycles. The group should stop all consumers if any one of them returns a fatal error.
- [ ] Develop a `Close()` method to trigger a graceful shutdown on all consumers in the group concurrently.

## Add 'Immediate Re-Queue' Acknowledgment Strategy

To provide users with more control over message retry speed, this feature will add an alternative acknowledgment strategy. Unlike the default "no-op" strategy which relies on the SQS visibility timeout, this new strategy will make a failed message immediately visible in the queue for a faster retry, which is ideal for transient errors.

- [ ] Create a new acknowledger implementation (e.g., `ImmediateAcknowledger`).
- [ ] Implement the `Reject` method within this new acknowledger to call the SQS `ChangeMessageVisibility` API, setting the `VisibilityTimeout` to `0`.
- [ ] Update the `sqsConnector` interface to include the `ChangeMessageVisibility` method signature.
- [ ] Add a configuration option (e.g., `WithAcknowledgmentStrategy()`) to allow users to select their preferred strategy when creating a new consumer.
- [ ] Document the new strategy and its trade-offs (faster retries vs. increased API call costs) for users.