package lambda

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// UpdateAlias points an existing alias of the function at the given published version.
// Anything that invokes the function through the alias starts using that version.
func UpdateAlias(ctx context.Context, infra Outputs, alias string, version string) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig(), StandardRetrierFn)
	_, err := λClient.UpdateAlias(ctx, &lambda.UpdateAliasInput{
		FunctionName:    aws.String(infra.FunctionName()),
		Name:            aws.String(alias),
		FunctionVersion: aws.String(version),
	})
	return err
}
