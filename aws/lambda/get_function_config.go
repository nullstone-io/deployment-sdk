package lambda

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func GetFunctionConfig(ctx context.Context, infra Outputs) (*lambda.GetFunctionConfigurationOutput, error) {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	return λClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(infra.FunctionName()),
	})
}
