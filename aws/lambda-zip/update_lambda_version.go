package lambda_zip

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func UpdateLambdaVersion(ctx context.Context, infra Outputs, version string) error {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())
	_, err := λClient.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(infra.LambdaName),
		DryRun:       false,
		Publish:      false,
		S3Bucket:     aws.String(infra.ArtifactsBucketName),
		S3Key:        aws.String(infra.ArtifactsKey(version)),
	})
	return err
}
