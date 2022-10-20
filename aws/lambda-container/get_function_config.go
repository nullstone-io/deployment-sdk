package lambda_container

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetFunctionConfig(ctx context.Context, infra Outputs) (*lambda.GetFunctionConfigurationOutput, error) {
	λClient := lambda.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	return λClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(infra.LambdaName),
	})
}
