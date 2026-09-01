package iampropagation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bufOsWriters struct {
	buf bytes.Buffer
}

func (w *bufOsWriters) Stdout() io.Writer { return &w.buf }
func (w *bufOsWriters) Stderr() io.Writer { return &w.buf }

func TestIsPropagationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ecr GetAuthorizationToken access denied",
			err: fmt.Errorf("operation error ECR: GetAuthorizationToken: %w",
				&smithy.GenericAPIError{Code: "AccessDeniedException", Message: "User: arn:aws:sts::123:assumed-role/pusher is not authorized to perform: ecr:GetAuthorizationToken"}),
			want: true,
		},
		{
			name: "docker push registry denial",
			err:  errors.New("denied: User: arn:aws:sts::123:assumed-role/pusher is not authorized to perform: ecr:InitiateLayerUpload"),
			want: true,
		},
		{
			name: "sts invalid client token",
			err:  &smithy.GenericAPIError{Code: "InvalidClientTokenId", Message: "The security token included in the request is invalid"},
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, IsPropagationError(test.err))
		})
	}
}

func TestRetry(t *testing.T) {
	accessDenied := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized"}

	t.Run("non-propagation errors fail immediately", func(t *testing.T) {
		osWriters := &bufOsWriters{}
		boom := errors.New("boom")
		calls := 0
		err := Retry(context.Background(), osWriters, "pushing", time.Minute, func(ctx context.Context) error {
			calls++
			return boom
		})
		assert.ErrorIs(t, err, boom)
		assert.Equal(t, 1, calls)
		assert.Empty(t, osWriters.buf.String())
	})

	t.Run("retries propagation error until success and logs why", func(t *testing.T) {
		osWriters := &bufOsWriters{}
		calls := 0
		err := Retry(context.Background(), osWriters, "pushing the image to ECR", time.Minute, func(ctx context.Context) error {
			calls++
			if calls == 1 {
				return accessDenied
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 2, calls)
		assert.Contains(t, osWriters.buf.String(), "Access denied while pushing the image to ECR")
		assert.Contains(t, osWriters.buf.String(), "eventually consistent")
		assert.Contains(t, osWriters.buf.String(), "Retrying in")
	})

	t.Run("gives up when the window is exhausted", func(t *testing.T) {
		osWriters := &bufOsWriters{}
		calls := 0
		err := Retry(context.Background(), osWriters, "pushing", 0, func(ctx context.Context) error {
			calls++
			return accessDenied
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, error(accessDenied))
		assert.Contains(t, err.Error(), "access denied while pushing after retrying")
		assert.Equal(t, 1, calls)
	})
}
