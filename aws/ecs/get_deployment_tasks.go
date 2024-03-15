package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetDeploymentTasks(ctx context.Context, infra Outputs, deploymentId string) ([]ecstypes.Task, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(infra.ClusterArn()),
		ServiceName: aws.String(infra.ServiceName),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get tasks associated with service (%s): %w", infra.ServiceName, err)
	}

	// if there aren't any tasks returned, we can't fetch any task descriptions
	if len(out.TaskArns) == 0 {
		return nil, nil
	}

	out2, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(infra.ClusterArn()),
		Tasks:   out.TaskArns,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get task details: %w", err)
	}

	// For ECS/Fargate services, StartedBy is set to the ECS deployment ID
	// This won't work for RunTask because StartedBy is manually set during that operation
	tasks := make([]ecstypes.Task, 0)
	for _, task := range out2.Tasks {
		if task.StartedBy != nil && *task.StartedBy == deploymentId {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}
