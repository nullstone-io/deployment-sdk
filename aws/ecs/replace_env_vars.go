package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/otel"
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

// ApplyUserEnvVars upserts user-supplied (deploy-time) env vars into every container definition.
// Unlike ReplaceEnvVars, this adds env vars that don't already exist in addition to overriding existing ones.
// Values are interpolated against other user env vars and the standard env vars (see env_vars.ResolveUser).
func ApplyUserEnvVars(taskDef *types.TaskDefinition, meta app.DeployMetadata) bool {
	userEnvVars := env_vars.ResolveUser(meta)
	if len(userEnvVars) == 0 {
		return false
	}

	for i, cd := range taskDef.ContainerDefinitions {
		for name, value := range userEnvVars {
			cd.Environment = upsertEnvVar(cd.Environment, name, value)
		}
		taskDef.ContainerDefinitions[i] = cd
	}
	return true
}

func upsertEnvVar(kvps []types.KeyValuePair, name, value string) []types.KeyValuePair {
	for i, kvp := range kvps {
		if kvp.Name != nil && *kvp.Name == name {
			kvps[i].Value = aws.String(value)
			return kvps
		}
	}
	return append(kvps, types.KeyValuePair{Name: aws.String(name), Value: aws.String(value)})
}

func ReplaceOtelResourceAttributesEnvVar(taskDef *types.TaskDefinition, meta app.DeployMetadata) bool {
	fn := otel.UpdateResourceAttributes(meta.Version, meta.CommitSha, false)

	for i, cd := range taskDef.ContainerDefinitions {
		for j, kvp := range cd.Environment {
			if kvp.Name != nil && *kvp.Name == otel.ResourceAttributesEnvName && kvp.Value != nil {
				kvp.Value = aws.String(fn(*kvp.Value))
				cd.Environment[j] = kvp
				taskDef.ContainerDefinitions[i] = cd
				return true
			}
		}
	}

	return false
}
