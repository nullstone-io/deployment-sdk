package lambda

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/sethvargo/go-retry"
)

var (
	ErrTimeoutWaitingForVersion = errors.New("AWS is taking a long time to prepare the published lambda version. Please retry deployment.")
	ErrFailedVersion            = errors.New("AWS failed to prepare the published lambda version.")

	// errVersionPending is returned (as a retryable error) while AWS is still preparing the version.
	errVersionPending = errors.New("lambda version is still being prepared")
)

// versionReadiness is the outcome of inspecting a published version's state
type versionReadiness int

const (
	versionReady versionReadiness = iota
	versionPending
	versionFailed
)

// evaluateVersionState decides whether a published version is ready to receive traffic.
// See https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html
// With SnapStart, a version is not usable until AWS has initialized the function and taken its snapshot;
// invocations against the version (and pointing an alias at it) fail until then.
//
// AWS reports progress through two fields and we honor both: State (Pending/Active/Failed) and
// LastUpdateStatus (InProgress/Successful/Failed). A failure in either is a failure; otherwise, work
// in progress in either means the version is not ready yet.
func evaluateVersionState(state types.State, lastUpdateStatus types.LastUpdateStatus) versionReadiness {
	if state == types.StateFailed || lastUpdateStatus == types.LastUpdateStatusFailed {
		return versionFailed
	}
	if state == types.StatePending || lastUpdateStatus == types.LastUpdateStatusInProgress {
		return versionPending
	}
	// Active and Inactive (idle, reactivated on next invoke) are both usable.
	// AWS omits these fields for functions that have never left Active.
	return versionReady
}

// WaitForVersionActive waits for AWS to finish preparing a published version of a function.
// It returns immediately if the version is already usable.
// heartbeatFn is called on each poll while AWS is still preparing the version; it receives the elapsed
// time since the wait started so callers can report progress.
func WaitForVersionActive(ctx context.Context, infra Outputs, version string, timeout time.Duration, heartbeatFn func(elapsed time.Duration)) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	start := time.Now()
	b := retry.WithMaxDuration(timeout, retry.NewConstant(5*time.Second))
	err := retry.Do(ctx, b, func(ctx context.Context) error {
		out, err := λClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
			FunctionName: aws.String(infra.FunctionName()),
			Qualifier:    aws.String(version),
		})
		if err != nil {
			return err
		}
		switch evaluateVersionState(out.State, out.LastUpdateStatus) {
		case versionPending:
			heartbeatFn(time.Since(start).Round(time.Second))
			return retry.RetryableError(errVersionPending)
		case versionFailed:
			// The reason explains why AWS could not prepare the version (e.g. function initialization failed)
			reason := aws.ToString(out.StateReason)
			if reason == "" {
				reason = aws.ToString(out.LastUpdateStatusReason)
			}
			if reason != "" {
				return fmt.Errorf("%w (reason: %s)", ErrFailedVersion, reason)
			}
			return ErrFailedVersion
		}
		return nil
	})
	// When the retry budget is exhausted, AWS was still preparing the version; surface a timeout error.
	if errors.Is(err, errVersionPending) {
		return ErrTimeoutWaitingForVersion
	}
	return err
}
