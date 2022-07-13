package lambda_zip

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func UpdateLambdaVersion(ctx context.Context, infra Outputs, version string) error {
	λClient := lambda.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	_, err := λClient.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(infra.LambdaName),
		DryRun:       false,
		Publish:      false,
		S3Bucket:     aws.String(infra.ArtifactsBucketName),
		S3Key:        aws.String(infra.ArtifactsKey(version)),
	})
	return err
}
