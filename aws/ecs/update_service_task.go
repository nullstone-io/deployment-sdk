package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
	"sort"
)

func UpdateServiceTask(ctx context.Context, infra Outputs, newTaskDefArn string) (*ecstypes.Deployment, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	out, err := ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
		Service:            aws.String(infra.ServiceName),
		Cluster:            aws.String(infra.ClusterArn()),
		ForceNewDeployment: true,
		TaskDefinition:     aws.String(newTaskDefArn),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to update service task: %w", err)
	}

	deployments := out.Service.Deployments
	sort.SliceStable(deployments, func(i, j int) bool {
		return deployments[i].CreatedAt.After(*deployments[j].CreatedAt)
	})
	for _, deployment := range deployments {
		if *deployment.TaskDefinition == newTaskDefArn {
			return &deployment, nil
		}
	}

	return nil, fmt.Errorf("unable to find the deployment associated with the updated service task: %s", newTaskDefArn)
}
