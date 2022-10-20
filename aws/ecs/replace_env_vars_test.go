package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplaceEnvVars(t *testing.T) {
	taskDef := types.TaskDefinition{
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
					{Name: aws.String("NULLSTONE_VERSION"), Value: aws.String("0.1.0")},
					{Name: aws.String("NULLSTONE_COMMIT_SHA"), Value: aws.String("2222d5fc")},
				},
			},
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
					{Name: aws.String("NULLSTONE_VERSION"), Value: aws.String("0.1.0")},
				},
			},
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
				},
			},
		},
	}
	meta := app.DeployMetadata{
		Repo:        "",
		Version:     "0.2.0",
		CommitSha:   "1ab2d5fc",
		Type:        "",
		PackageMode: "",
	}

	want := types.TaskDefinition{
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
					{Name: aws.String("NULLSTONE_VERSION"), Value: aws.String("0.2.0")},
					{Name: aws.String("NULLSTONE_COMMIT_SHA"), Value: aws.String("1ab2d5fc")},
				},
			},
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
					{Name: aws.String("NULLSTONE_VERSION"), Value: aws.String("0.2.0")},
				},
			},
			{
				Environment: []types.KeyValuePair{
					{Name: aws.String("ANOTHER"), Value: aws.String("value")},
				},
			},
		},
	}
	got := ReplaceEnvVars(taskDef, meta)
	assert.Equal(t, want, *got)
}
