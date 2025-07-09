package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func UpdateTask(ctx context.Context, infra Outputs, taskDefinition *ecstypes.TaskDefinition, taskDefTags []ecstypes.Tag, previousTaskDefArn string) (*ecstypes.TaskDefinition, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    taskDefinition.ContainerDefinitions,
		Family:                  taskDefinition.Family,
		Cpu:                     taskDefinition.Cpu,
		ExecutionRoleArn:        taskDefinition.ExecutionRoleArn,
		EphemeralStorage:        taskDefinition.EphemeralStorage,
		InferenceAccelerators:   taskDefinition.InferenceAccelerators,
		IpcMode:                 taskDefinition.IpcMode,
		Memory:                  taskDefinition.Memory,
		NetworkMode:             taskDefinition.NetworkMode,
		PidMode:                 taskDefinition.PidMode,
		PlacementConstraints:    taskDefinition.PlacementConstraints,
		ProxyConfiguration:      taskDefinition.ProxyConfiguration,
		RequiresCompatibilities: taskDefinition.RequiresCompatibilities,
		RuntimePlatform:         taskDefinition.RuntimePlatform,
		TaskRoleArn:             taskDefinition.TaskRoleArn,
		Volumes:                 taskDefinition.Volumes,
		Tags:                    taskDefTags,
	}
	out, err := ecsClient.RegisterTaskDefinition(ctx, input)
	if err != nil {
		return nil, err
	}

	_, err = ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: &previousTaskDefArn,
	})
	if err != nil {
		return nil, err
	}

	return out.TaskDefinition, nil
}
