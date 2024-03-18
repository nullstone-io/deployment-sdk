package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"slices"
	"sort"
)

func GetAllDeploymentTaskArns(ctx context.Context, infra Outputs, deploymentId string) ([]string, error) {
	allTaskArns := make([]string, 0)

	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out1, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(infra.ClusterArn()),
		StartedBy:     aws.String(deploymentId),
		DesiredStatus: types.DesiredStatusRunning,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting running deployment tasks: %w", err)
	}
	allTaskArns = append(allTaskArns, out1.TaskArns...)

	out2, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(infra.ClusterArn()),
		StartedBy:     aws.String(deploymentId),
		DesiredStatus: types.DesiredStatusStopped,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting stopped deployment tasks: %w", err)
	}
	allTaskArns = append(allTaskArns, out2.TaskArns...)

	sort.Strings(allTaskArns)
	slices.Compact(allTaskArns)
	return allTaskArns, nil
}
