package beanstalk

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetApplicationVersion(ctx context.Context, infra Outputs, version string) (*ebtypes.ApplicationVersionDescription, error) {
	bclient := elasticbeanstalk.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := bclient.DescribeApplicationVersions(ctx, &elasticbeanstalk.DescribeApplicationVersionsInput{
		ApplicationName: aws.String(infra.BeanstalkName),
		VersionLabels:   []string{version},
	})
	if err != nil {
		return nil, err
	}
	for _, appVersion := range out.ApplicationVersions {
		return &appVersion, nil
	}
	return nil, nil
}
