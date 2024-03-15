package ecs

import (
	"context"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type TaskDefinitionsCache map[string]*ecstypes.TaskDefinition

func (c TaskDefinitionsCache) Get(ctx context.Context, infra Outputs, taskDefArn *string) (*ecstypes.TaskDefinition, error) {
	if taskDefArn == nil {
		return nil, nil
	}
	if def, ok := c[*taskDefArn]; ok {
		return def, nil
	}

	def, err := GetTaskDefinition(ctx, infra)
	if err != nil {
		return nil, err
	}
	c[*taskDefArn] = def
	return def, nil
}
