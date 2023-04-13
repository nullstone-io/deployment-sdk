package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func DescribeTasks(ctx context.Context, infra Outputs, taskArns []string) ([]ecstypes.Task, error) {
	if len(taskArns) == 0 {
		return []ecstypes.Task{}, nil
	}

	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(infra.ClusterArn()),
		Tasks:   taskArns,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get task details: %w", err)
	}

	return out.Tasks, nil
}
