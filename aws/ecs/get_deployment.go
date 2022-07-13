package ecs

import (
	"context"
	"fmt"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func GetDeployment(ctx context.Context, infra Outputs, deploymentId string) (*ecstypes.Deployment, error) {
	svc, err := GetService(ctx, infra)
	if err != nil {
		return nil, fmt.Errorf("error retrieving fargate service: %w", err)
	}

	for _, deployment := range svc.Deployments {
		if *deployment.Id == deploymentId {
			return &deployment, nil
		}
	}
	return nil, fmt.Errorf("no deployments returned with an id of %s", deploymentId)
}
