package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetTasksWithDetail(ctx context.Context, infra Outputs, taskArns []string) ([]types.Task, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	if len(taskArns) == 0 {
		return nil, nil
	}

	out, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(infra.ClusterArn()),
		Tasks:   taskArns,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get task details: %w", err)
	}

	return out.Tasks, nil
}
