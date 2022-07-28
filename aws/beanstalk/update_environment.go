package beanstalk

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func UpdateEnvironment(ctx context.Context, infra Outputs, version string) error {
	bclient := elasticbeanstalk.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	_, err := bclient.UpdateEnvironment(ctx, &elasticbeanstalk.UpdateEnvironmentInput{
		ApplicationName: aws.String(infra.BeanstalkName),
		EnvironmentId:   aws.String(infra.EnvironmentId),
		VersionLabel:    aws.String(version),
	})
	return err
}

