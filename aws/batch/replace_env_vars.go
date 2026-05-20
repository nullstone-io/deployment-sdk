package batch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/otel"
)

// ReplaceEnvVars updates the container definition with updated env vars as a result of the deploy
// This runs through the env vars and replaces the standard env vars with the updated values
func ReplaceEnvVars(jobDef batchtypes.JobDefinition, meta app.DeployMetadata) batchtypes.JobDefinition {
	std := env_vars.GetStandard(meta)

	updateEnv := func(kvps []batchtypes.KeyValuePair) []batchtypes.KeyValuePair {
		updated := make([]batchtypes.KeyValuePair, 0)
		for _, kvp := range kvps {
			if val, ok := std[*kvp.Name]; ok {
				// We found an env var that's in our list of standard env vars, replace it
				kvp.Value = aws.String(val)
			}
			updated = append(updated, batchtypes.KeyValuePair{
				Name:  kvp.Name,
				Value: kvp.Value,
			})
		}
		return updated
	}

	jobDef.ContainerProperties.Environment = updateEnv(jobDef.ContainerProperties.Environment)

	return jobDef
}

// ApplyUserEnvVars upserts user-supplied (deploy-time) env vars into the job definition's container.
// Unlike ReplaceEnvVars, this adds env vars that don't already exist in addition to overriding existing ones.
func ApplyUserEnvVars(jobDef *batchtypes.JobDefinition, meta app.DeployMetadata) bool {
	userEnvVars := env_vars.ResolveUser(meta)
	if len(userEnvVars) == 0 {
		return false
	}

	for name, value := range userEnvVars {
		jobDef.ContainerProperties.Environment = upsertEnvVar(jobDef.ContainerProperties.Environment, name, value)
	}
	return true
}

func upsertEnvVar(kvps []batchtypes.KeyValuePair, name, value string) []batchtypes.KeyValuePair {
	for i, kvp := range kvps {
		if kvp.Name != nil && *kvp.Name == name {
			kvps[i].Value = aws.String(value)
			return kvps
		}
	}
	return append(kvps, batchtypes.KeyValuePair{Name: aws.String(name), Value: aws.String(value)})
}

func ReplaceOtelResourceAttributesEnvVar(jobDef *batchtypes.JobDefinition, meta app.DeployMetadata) bool {
	fn := otel.UpdateResourceAttributes(meta.Version, meta.CommitSha, false)

	for i, kvp := range jobDef.ContainerProperties.Environment {
		if kvp.Name != nil && *kvp.Name == otel.ResourceAttributesEnvName && kvp.Value != nil {
			kvp.Value = aws.String(fn(*kvp.Value))
			jobDef.ContainerProperties.Environment[i] = kvp
			return true
		}
	}

	return false
}
