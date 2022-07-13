package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetTaskDefinition(ctx context.Context, infra Outputs) (*ecstypes.TaskDefinition, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	out2, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(infra.TaskArn),
	})
	if err != nil {
		return nil, err
	}
	return out2.TaskDefinition, nil
}
