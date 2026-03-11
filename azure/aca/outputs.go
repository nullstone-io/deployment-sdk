package aca

import (
	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	SubscriptionId    string          `ns:"subscription_id"`
	ResourceGroup     string          `ns:"resource_group"`
	ContainerAppName  string          `ns:"container_app_name,optional"`
	JobName           string          `ns:"job_name,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	Deployer          azure.Principal `ns:"deployer"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.InitializeCreds(source, ws, "deployer")
}
