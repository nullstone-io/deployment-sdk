package lambda

import (
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

// StandardRetrierFn configures a new aws sdk client with a retrier that ensures function code and configuration are updated properly
// AWS is eventually-consistent, so if you update function code immediately after function configuration because the function is still updating
// This retryer will appropriately handle the AWS failure (i.e. ResourceConflictException) and retry
// By default, this is configured to run 5 attempts with a total delay of 0 to 50s (uses random, increasing jitter between each attempt)
func StandardRetrierFn(options *lambda.Options) {
	options.Retryer = NewDeployRetrier(5)
}

func NewDeployRetrier(maxAttempts int) aws.Retryer {
	resourceConflictErr := types.ResourceConflictException{}

	return retry.NewStandard(func(options *retry.StandardOptions) {
		options.MaxAttempts = maxAttempts
		options.Retryables = append(options.Retryables, nsaws.AnonRetryable(func(err error) aws.Ternary {
			var ae smithy.APIError
			if errors.As(err, &ae) && ae.ErrorCode() == resourceConflictErr.ErrorCode() {
				return aws.TrueTernary
			}
			return aws.UnknownTernary
		}))
	})
}
