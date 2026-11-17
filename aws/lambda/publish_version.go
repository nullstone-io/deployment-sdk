package lambda

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// PublishVersion snapshots the function's current code and configuration ($LATEST) as an immutable, numbered version.
// It returns the version number.
// If nothing has changed since the last published version, AWS returns that version instead of creating a new one.
//
// A published version is not necessarily usable when this returns; use WaitForVersionActive.
func PublishVersion(ctx context.Context, infra Outputs, description string) (string, error) {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig(), StandardRetrierFn)
	input := &lambda.PublishVersionInput{
		FunctionName: aws.String(infra.FunctionName()),
	}
	if description != "" {
		input.Description = aws.String(description)
	}
	out, err := λClient.PublishVersion(ctx, input)
	if err != nil {
		return "", err
	}
	version := aws.ToString(out.Version)
	if version == "" {
		return "", fmt.Errorf("AWS did not report a version number for the published version")
	}
	return version, nil
}
