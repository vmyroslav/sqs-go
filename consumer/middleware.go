package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func NewTimeTrackingMiddleware[T any]() Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			start := time.Now()

			err := next.Handle(ctx, msg)

			elapsed := time.Since(start)
			fmt.Printf("Message processed in %s\n", elapsed)

			return err
		}
	}
}

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

func MiddlewareAdapter[T any](mw Middleware[any]) Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, msg T) error {
			// Create a new handler that operates on T
			specificHandler := func(ctx context.Context, msg T) error {
				// Call the original handler with the specific message
				return next.Handle(ctx, msg)
			}

			// Create a new handler that matches the HandlerFunc[Message] type
			genericHandler := func(ctx context.Context, msg any) error {
				// Convert msg from Message to T
				specificMsg, ok := msg.(T)
				if !ok {
					return fmt.Errorf("unexpected message type: %T", msg)
				}

				// Call specificHandler with the specific message
				return specificHandler(ctx, specificMsg)
			}

			// Call the original middleware with the generic handler and message
			return mw(genericHandler)(ctx, msg)
		}
	}
}
