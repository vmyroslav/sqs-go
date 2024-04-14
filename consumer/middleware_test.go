package consumer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHandler struct {
	err error
}

func (m *mockHandler) Handle(_ context.Context, _ interface{}) error {
	return m.err
}

func TestNewIgnoreErrorsMiddleware(t *testing.T) {
	t.Parallel()

	middleware := NewIgnoreErrorsMiddleware[any](nil)

	t.Run("handler returns error", func(t *testing.T) {
		t.Parallel()

		handler := &mockHandler{err: fmt.Errorf("test error")}
		mwHandler := middleware(handler.Handle)

		err := mwHandler(context.Background(), "test message")
		require.NoError(t, err)
	})

	t.Run("handler returns no error", func(t *testing.T) {
		t.Parallel()

		handler := &mockHandler{err: nil}
		mwHandler := middleware(handler.Handle)

		err := mwHandler(context.Background(), "test message")
		require.NoError(t, err)
	})
}

func TestMiddlewareAdapter(t *testing.T) {
	t.Parallel()

	type customMsg struct {
		Payload string
	}

	t.Run("correct message type", func(t *testing.T) {
		handler := HandlerFunc[customMsg](func(ctx context.Context, msg customMsg) error {
			return nil
		})

		receivedCh := make(chan struct{}, 1)
		defer close(receivedCh)

		m := func(next HandlerFunc[any]) HandlerFunc[any] {
			return func(ctx context.Context, msg any) error {
				receivedCh <- struct{}{}
				return next.Handle(ctx, msg)
			}
		}

		// mm := m(handler) cannot use handler (type HandlerFunc[customMsg]) as type HandlerFunc[any] in argument to m

		mwHandler := MiddlewareAdapter[customMsg](m)(handler)
		err := mwHandler(context.Background(), customMsg{Payload: "test message"})
		require.NoError(t, err)
		assert.NotNil(t, <-receivedCh)
	})
}

func TestNewPanicRecoverMiddleware(t *testing.T) {
	tests := []struct {
		name    string
		handler HandlerFunc[any]
		wantErr bool
	}{
		{
			name: "handler panics",
			handler: func(ctx context.Context, msg any) error {
				panic("test panic")
			},
			wantErr: true,
		},
		{
			name: "handler does not panic",
			handler: func(ctx context.Context, msg any) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewPanicRecoverMiddleware[any]()
			mwHandler := middleware(tt.handler)

			err := mwHandler(context.Background(), "test message")

			if (err != nil) != tt.wantErr {
				t.Errorf("NewPanicRecoverMiddleware() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err == nil {
				t.Errorf("Expected an error but got nil")
			}
		})
	}
}

func TestNewTimeLimitMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler HandlerFunc[any]
		timeout time.Duration
		wantErr bool
	}{
		{
			name: "handler exceeds timeout",
			handler: func(ctx context.Context, msg any) error {
				time.Sleep(300 * time.Millisecond)
				return nil
			},
			timeout: 100 * time.Millisecond,
			wantErr: true,
		},
		{
			name: "handler does not exceed timeout",
			handler: func(ctx context.Context, msg any) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			},
			timeout: 100 * time.Millisecond,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware := NewTimeLimitMiddleware[any](tt.timeout)
			mwHandler := middleware(tt.handler)

			err := mwHandler(context.Background(), "test message")

			if (err != nil) != tt.wantErr {
				t.Errorf("NewTimeLimitMiddleware() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err == nil {
				t.Errorf("Expected an error but got nil")
			}
		})
	}
}
