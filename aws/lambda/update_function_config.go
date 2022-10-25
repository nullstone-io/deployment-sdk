package lambda

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func UpdateFunctionConfig(ctx context.Context, infra Outputs, config *lambda.UpdateFunctionConfigurationInput) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	_, err := λClient.UpdateFunctionConfiguration(ctx, config)
	return err
}
