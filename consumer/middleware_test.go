package consumer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHandler struct {
	err error
}

func (m *mockHandler) Handle(ctx context.Context, msg interface{}) error {
	return m.err
}

func TestNewIgnoreErrorsMiddleware(t *testing.T) {
	t.Parallel()

	middleware := NewIgnoreErrorsMiddleware[any](nil)

	t.Run("handler returns error", func(t *testing.T) {
		handler := &mockHandler{err: fmt.Errorf("test error")}
		mwHandler := middleware(handler.Handle)

		err := mwHandler(context.Background(), "test message")
		require.NoError(t, err)
	})

	t.Run("handler returns no error", func(t *testing.T) {
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

		var receivedCh = make(chan struct{}, 1)
		defer close(receivedCh)

		m := func(next HandlerFunc[any]) HandlerFunc[any] {
			return func(ctx context.Context, msg any) error {
				receivedCh <- struct{}{}
				return next.Handle(ctx, msg)
			}
		}

		//mm := m(handler) cannot use handler (type HandlerFunc[customMsg]) as type HandlerFunc[any] in argument to m

		mwHandler := MiddlewareAdapter[customMsg](m)(handler)
		err := mwHandler(context.Background(), customMsg{Payload: "test message"})
		require.NoError(t, err)
		assert.NotNil(t, <-receivedCh)
	})
}
