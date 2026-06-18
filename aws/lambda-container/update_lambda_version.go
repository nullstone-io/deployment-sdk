package lambda_container

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// UpdateLambdaVersion points the lambda function at the container image for the given version.
// It returns the fully-qualified image URI that was deployed so callers can report it.
func UpdateLambdaVersion(ctx context.Context, infra Outputs, version string) (string, error) {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	imageUrl := infra.ImageRepoUrl
	imageUrl.Digest = ""
	imageUrl.Tag = version
	imageUri := imageUrl.String()
	_, err := λClient.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(infra.LambdaName),
		DryRun:       false,
		Publish:      false,
		ImageUri:     aws.String(imageUri),
	})
	return imageUri, err
}
