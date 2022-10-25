package lambda

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/sethvargo/go-retry"
	"time"
)

var (
	ErrTimeoutWaitingForChanges = errors.New("AWS is taking a long time to update the lambda function. Please retry deployment.")
	ErrFailedLastChanges        = errors.New("The most recent changes to lambda failed to apply. Please retry deployment.")
)

// WaitForFunctionChanges waits for AWS to finalize changes for a function (configuration or code)
// See https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html#functions-states-updating
// During my live tests, this process took <30s
func WaitForFunctionChanges(ctx context.Context, infra Outputs, timeout time.Duration, heartbeatFn func()) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	b := retry.WithMaxDuration(timeout, retry.NewConstant(5*time.Second))
	return retry.Do(ctx, b, func(ctx context.Context) error {
		out, err := λClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
			FunctionName: aws.String(infra.FunctionName()),
		})
		if err != nil {
			return err
		} else if out != nil {
			switch out.LastUpdateStatus {
			case types.LastUpdateStatusInProgress:
				heartbeatFn()
				return retry.RetryableError(fmt.Errorf("lambda function is currently making changes"))
			case types.LastUpdateStatusFailed:
				return ErrFailedLastChanges
			case types.LastUpdateStatusSuccessful:
				return nil
			}
		}
		return nil
	})
}
