package ecs

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/docker"
)

func ReplaceTaskImageTag(infra Outputs, taskDefinition ecstypes.TaskDefinition, imageTag string) (*ecstypes.TaskDefinition, error) {
	defIndex, err := findMainContainerDefinitionIndex(infra.MainContainerName, taskDefinition.ContainerDefinitions)
	if err != nil {
		return nil, err
	}

	existingImageUrl := docker.ParseImageUrl(*taskDefinition.ContainerDefinitions[defIndex].Image)
	existingImageUrl.Digest = ""
	existingImageUrl.Tag = imageTag
	taskDefinition.ContainerDefinitions[defIndex].Image = aws.String(existingImageUrl.String())

	return &taskDefinition, nil
}

func findMainContainerDefinitionIndex(mainContainerName string, containerDefs []ecstypes.ContainerDefinition) (int, error) {
	if len(containerDefs) == 0 {
		return -1, fmt.Errorf("cannot deploy service with no container definitions")
	}
	if len(containerDefs) == 1 {
		return 0, nil
	}

	if mainContainerName != "" {
		// let's go find main_container_name
		for i, cd := range containerDefs {
			if cd.Name != nil && *cd.Name == mainContainerName {
				return i, nil
			}
		}
		return -1, fmt.Errorf("cannot deploy service; no container definition with main_container_name = %s", mainContainerName)
	}

	// main_container_name was not specified, we are going to attempt to find a single container definition
	// If more than one container definition exists, we will error
	if len(containerDefs) > 1 {
		return -1, fmt.Errorf("service contains multiple containers; cannot deploy unless service module exports 'main_container_name'")
	}
	return 0, nil
}
