// Package iampropagation retries AWS operations that fail with access-denied errors immediately
// after infrastructure is provisioned.
//
// AWS IAM is eventually consistent: an IAM role/policy created during an infra launch can take
// tens of seconds (occasionally minutes) to propagate to every AWS service endpoint. When a push
// runs concurrently with the first launch of an app, the freshly-created pusher identity can be
// rejected (e.g. `AccessDeniedException` on `ecr:GetAuthorizationToken`) even though the
// permissions are correctly defined. Retrying the whole operation with a generous window (and
// telling the user why we're waiting) turns those transient denials into successful pushes.
package iampropagation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/logging"
)

const (
	// DefaultWindow is the total time we allow for IAM permissions to propagate.
	// IAM propagation is usually <1m, but can occasionally take a few minutes.
	DefaultWindow = 3 * time.Minute

	initialDelay = 2 * time.Second
	maxDelay     = 30 * time.Second
)

// IsPropagationError reports whether err looks like an AWS access-denied error.
// These are the errors produced when an IAM identity/policy exists but hasn't propagated yet;
// they are indistinguishable from a genuine permissions misconfiguration, so callers pair this
// with a bounded retry window instead of retrying forever.
func IsPropagationError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "InvalidClientTokenId", "UnrecognizedClientException":
			return true
		}
	}
	// docker push/pull surface registry authorization failures as plain-text errors
	// (e.g. `denied: User: arn:... is not authorized to perform: ecr:InitiateLayerUpload`)
	msg := err.Error()
	return strings.Contains(msg, "not authorized to perform") ||
		strings.Contains(msg, "denied: requested access to the resource is denied")
}

// Retry runs fn, retrying with exponential backoff for up to window as long as fn fails with an
// access-denied error (see IsPropagationError). Any other error fails immediately.
// Each retry logs to osWriters' stderr so the user can see that we're waiting on IAM propagation and why.
// action is a short present-tense description used in messages (e.g. "pushing the image to ECR").
func Retry(ctx context.Context, osWriters logging.OsWriters, action string, window time.Duration, fn func(ctx context.Context) error) error {
	stderr := osWriters.Stderr()
	deadline := time.Now().Add(window)
	delay := initialDelay
	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if !IsPropagationError(err) {
			return err
		}
		if remaining := time.Until(deadline); remaining < delay {
			return fmt.Errorf("access denied while %s after retrying for %s; if this app's infrastructure was just launched, AWS IAM may still be propagating permissions and retrying may succeed; otherwise, verify the pusher identity's IAM permissions: %w", action, window, err)
		}
		if attempt == 1 {
			fmt.Fprintln(stderr)
			colorstring.Fprintf(stderr, "[yellow]Access denied while %s.\n", action)
			colorstring.Fprintln(stderr, "[yellow]If this app's infrastructure was just launched, its IAM permissions were created moments ago and AWS IAM is eventually consistent. Waiting for AWS to propagate IAM permissions.")
		}
		colorstring.Fprintf(stderr, "[yellow]Retrying in %s (attempt %d, giving up in %s)...\n", delay, attempt, time.Until(deadline).Round(time.Second))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay *= 2; delay > maxDelay {
			delay = maxDelay
		}
	}
}
