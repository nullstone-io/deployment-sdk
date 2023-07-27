package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
	"log"
)

func GetServiceTasks(ctx context.Context, infra Outputs) ([]ecstypes.Task, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	tasks, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(infra.ClusterArn()),
		ServiceName: aws.String(infra.ServiceName),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get tasks associated with service (%s): %w", infra.ServiceName, err)
	}

	log.Printf("DEBUG: (cluster_arn: %s, service_name: %s) Found tasks: %#v\n", infra.ClusterArn(), infra.ServiceName, tasks)
	// if there aren't any tasks returned, we can't fetch any task descriptions
	if len(tasks.TaskArns) == 0 {
		return nil, nil
	}

	out, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(infra.ClusterArn()),
		Tasks:   tasks.TaskArns,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get task details: %w", err)
	}

	return out.Tasks, nil
}
