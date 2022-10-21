package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
)

// ReplaceEnvVars updates every container definition with updated env vars as a result of the deploy
// This runs through the env vars and replaces the standard env vars with the updated values
func ReplaceEnvVars(taskDef types.TaskDefinition, meta app.DeployMetadata) *types.TaskDefinition {
	std := env_vars.GetStandard(meta)

	updateEnv := func(kvps []types.KeyValuePair) []types.KeyValuePair {
		updated := make([]types.KeyValuePair, 0)
		for _, kvp := range kvps {
			if val, ok := std[*kvp.Name]; ok {
				// We found an env var that's in our list of standard env vars, replace it
				kvp.Value = aws.String(val)
			}
			updated = append(updated, types.KeyValuePair{
				Name:  kvp.Name,
				Value: kvp.Value,
			})
		}
		return updated
	}

	for i, cd := range taskDef.ContainerDefinitions {
		cd.Environment = updateEnv(cd.Environment)
		taskDef.ContainerDefinitions[i] = cd
	}

	return &taskDef
}
