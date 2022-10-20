package lambda_container

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func UpdateFunctionConfig(ctx context.Context, infra Outputs, config *lambda.UpdateFunctionConfigurationInput) error {
	λClient := lambda.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	_, err := λClient.UpdateFunctionConfiguration(ctx, config)
	return err
}
