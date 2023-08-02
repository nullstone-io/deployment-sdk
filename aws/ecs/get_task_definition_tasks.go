package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetTaskFamilyTasks(ctx context.Context, infra Outputs) ([]ecstypes.Task, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	maxResults := int32(10)
	runningTasks, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(infra.ClusterArn()),
		Family:        aws.String(infra.TaskFamily()),
		MaxResults:    &maxResults,
		DesiredStatus: ecstypes.DesiredStatusRunning,
	})
	stoppedTasks, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(infra.ClusterArn()),
		Family:        aws.String(infra.TaskFamily()),
		MaxResults:    &maxResults,
		DesiredStatus: ecstypes.DesiredStatusStopped,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get tasks associated with task family (%s): %w", infra.TaskFamily(), err)
	}

	// if there aren't any tasks returned, we can't fetch any task descriptions
	tasks := append(runningTasks.TaskArns, stoppedTasks.TaskArns...)
	if len(tasks) == 0 {
		return nil, nil
	}
	pagedTasks := tasks[:maxResults]

	out, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(infra.ClusterArn()),
		Tasks:   pagedTasks,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get task details: %w", err)
	}

	return out.Tasks, nil
}
