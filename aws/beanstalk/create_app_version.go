package beanstalk

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func CreateAppVersion(ctx context.Context, infra Outputs, version string) (string, error) {
	bclient := elasticbeanstalk.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := bclient.CreateApplicationVersion(ctx, &elasticbeanstalk.CreateApplicationVersionInput{
		ApplicationName:       aws.String(infra.BeanstalkName),
		VersionLabel:          aws.String(version),
		AutoCreateApplication: aws.Bool(false),
		Process:               aws.Bool(true),
		SourceBundle: &ebtypes.S3Location{
			S3Bucket: aws.String(infra.ArtifactsBucketName),
			S3Key:    aws.String(infra.ArtifactsKey(version)),
		},
	})
	if err != nil {
		return "", err
	}
	if out.ApplicationVersion == nil || out.ApplicationVersion.ApplicationVersionArn == nil {
		return "", nil
	}
	return *out.ApplicationVersion.ApplicationVersionArn, nil
}
