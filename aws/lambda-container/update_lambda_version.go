package lambda_container

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func UpdateLambdaVersion(ctx context.Context, infra Outputs, version string) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	imageUrl := infra.ImageRepoUrl
	imageUrl.Digest = ""
	imageUrl.Tag = version
	_, err := λClient.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(infra.LambdaName),
		DryRun:       false,
		Publish:      false,
		ImageUri:     aws.String(imageUrl.String()),
	})
	return err
}
