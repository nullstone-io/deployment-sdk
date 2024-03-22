package batch

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
)

type Outputs struct {
	Region            string          `ns:"region"`
	JobDefinitionArn  string          `ns:"job_definition_arn"`
	JobDefinitionName string          `ns:"job_definition_name"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher       nsaws.User      `ns:"image_pusher,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`
}
