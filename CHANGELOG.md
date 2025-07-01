# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.2.0] - 2025-07-01

### Added
- Test coverage for processor reject functionality
- Test coverage for config visibility timeout validation
- Manual release workflow with semantic versioning
- Improved CI workflow to prevent duplicate pipeline runs

### Changed
- Updated Go version to 1.24.x
- Updated dependencies to latest versions
- Improved test coverage from ~87% to ~88%

### Fixed
- Duplicate CI pipeline runs on PR branches
- Missing test coverage for error handling paths

## [v0.1.0] - Initial Release

### Added
- SQS Consumer library with generic type support
- Core components:
  - `SQSConsumer[T]` - Main consumer with configurable worker pools
  - `Handler[T]` - Generic message handler interface  
  - `MessageAdapter[T]` - Type-safe message transformation
  - `Middleware[T]` - Middleware chain support
- Message adapters:
  - `JSONMessageAdapter[T]` - JSON unmarshaling to typed structs
  - `DummyAdapter[T]` - Pass-through for raw SQS messages
- Configuration system with validation:
  - Worker pool sizes for polling and processing
  - SQS-specific settings (visibility timeout, wait time, batch size)
  - Error handling thresholds
  - Graceful shutdown timeout
- Built-in middleware:
  - `NewIgnoreErrorsMiddleware` - Error suppression
  - `NewPanicRecoverMiddleware` - Panic recovery
  - `NewTimeLimitMiddleware` - Execution time limits
- Comprehensive test suite with mocks
- GitHub Actions CI/CD pipeline
- Dependabot configuration for automated updates
- MIT License
- Documentation and examples

### Technical Features
- Generic type system for type-safe message handling
- Concurrent message processing with configurable worker pools
- Graceful shutdown support
- Message acknowledgment and rejection handling
- Configurable retry mechanisms
- AWS SQS integration with proper error handling
- Mock generation for testing (`//go:generate mockery`)

### Dependencies
- AWS SDK Go v2 for SQS operations
- Testify for testing framework
- Task runner for build automation
- golangci-lint for code quality

[Unreleased]: https://github.com/vmyroslav/sqs-go/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/vmyroslav/sqs-go/releases/tag/v0.1.0