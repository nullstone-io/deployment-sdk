package batch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/nullstone-io/deployment-sdk/docker"
)

func ReplaceJobDefinitionImageTag(infra Outputs, jobDefinition batchtypes.JobDefinition, imageTag string) batchtypes.JobDefinition {
	existingImageUrl := docker.ParseImageUrl(*jobDefinition.ContainerProperties.Image)
	existingImageUrl.Digest = ""
	existingImageUrl.Tag = imageTag
	jobDefinition.ContainerProperties.Image = aws.String(existingImageUrl.String())

	return jobDefinition
}
