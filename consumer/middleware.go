package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// NewIgnoreErrorsMiddleware creates a new middleware that ignores errors that occur during message processing.
// If the logger is provided, it will log the error.
func NewIgnoreErrorsMiddleware[T any](l *slog.Logger) Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			err := next.Handle(ctx, msg)
			if err != nil && l != nil {
				l.ErrorContext(ctx, fmt.Sprintf("failed to process message: %v", err))
			}

			return nil
		}
	}
}

// NewPanicRecoverMiddleware creates a middleware that recovers from panics during message processing
func NewPanicRecoverMiddleware[T any]() Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("recovered from panic: %v", r)
				}
			}()

			return next.Handle(ctx, msg)
		}
	}
}

// NewTimeLimitMiddleware creates a middleware that enforces a timeout on message processing
func NewTimeLimitMiddleware[T any](timeout time.Duration) Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan error)

			go func() {
				done <- next(ctx, msg)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// MiddlewareAdapter adapts a middleware of any type to a middleware of a specific type T.
// It creates a new handler that operates on T and a new handler that matches the HandlerFunc[Message] type.
func MiddlewareAdapter[T any](mw Middleware[any]) Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			// create a new handler that operates on T
			specificHandler := func(ctx context.Context, msg T) error {
				// Call the original handler with the specific message
				return next.Handle(ctx, msg)
			}

			// create a new handler that matches the HandlerFunc[Message] type
			genericHandler := func(ctx context.Context, msg any) error {
				// convert msg from Message to T
				specificMsg, ok := msg.(T)
				if !ok {
					return fmt.Errorf("unexpected message type: %T", msg)
				}

				return specificHandler(ctx, specificMsg)
			}

			return mw(genericHandler)(ctx, msg)
		}
	}
}
