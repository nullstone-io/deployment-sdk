package beanstalk

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetEnvironmentStatus(ctx context.Context, infra Outputs, reference string) (*types.EnvironmentDescription, error) {
	bclient := elasticbeanstalk.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := bclient.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{
		ApplicationName: aws.String(infra.BeanstalkName),
		EnvironmentIds:  []string{infra.EnvironmentId},
		VersionLabel:    aws.String(reference),
	})
	if err != nil {
		return nil, err
	}
	if out == nil || out.Environments == nil || len(out.Environments) < 1 {
		return nil, nil
	}
	e := out.Environments[0]
	return &e, nil
}
