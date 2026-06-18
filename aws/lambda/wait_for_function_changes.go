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
	ErrTimeoutWaitingForChanges = errors.New("AWS is taking a long time to update the lambda function. Please retry deployment.")
	ErrFailedLastChanges        = errors.New("The most recent changes to lambda failed to apply. Please retry deployment.")

	// errChangesInProgress is returned (as a retryable error) while AWS is still applying changes.
	// When the retry budget is exhausted, retry.Do surfaces this error, which we translate into
	// ErrTimeoutWaitingForChanges so callers can detect a timeout with errors.Is.
	errChangesInProgress = errors.New("lambda function is currently making changes")
)

// WaitForFunctionChanges waits for AWS to finalize changes for a function (configuration or code)
// See https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html#functions-states-updating
// During my live tests, this process took <30s (this is longer when deploying larger docker images)
// heartbeatFn is called on each poll while AWS is still applying changes; it receives the elapsed
// time since the wait started so callers can report progress.
func WaitForFunctionChanges(ctx context.Context, infra Outputs, timeout time.Duration, heartbeatFn func(elapsed time.Duration)) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	start := time.Now()
	b := retry.WithMaxDuration(timeout, retry.NewConstant(5*time.Second))
	err := retry.Do(ctx, b, func(ctx context.Context) error {
		out, err := λClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
			FunctionName: aws.String(infra.FunctionName()),
		})
		if err != nil {
			return err
		} else if out != nil {
			switch out.LastUpdateStatus {
			case types.LastUpdateStatusInProgress:
				heartbeatFn(time.Since(start).Round(time.Second))
				return retry.RetryableError(errChangesInProgress)
			case types.LastUpdateStatusFailed:
				// LastUpdateStatusReason explains why AWS rejected the change (e.g. an invalid image)
				if reason := aws.ToString(out.LastUpdateStatusReason); reason != "" {
					return fmt.Errorf("%w (reason: %s)", ErrFailedLastChanges, reason)
				}
				return ErrFailedLastChanges
			case types.LastUpdateStatusSuccessful:
				return nil
			}
		}
		return nil
	})
	// When the retry budget is exhausted, AWS was still applying changes; surface a timeout error.
	if errors.Is(err, errChangesInProgress) {
		return ErrTimeoutWaitingForChanges
	}
	return err
}
