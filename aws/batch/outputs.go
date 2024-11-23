package batch

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region            string          `ns:"region"`
	JobDefinitionArn  string          `ns:"job_definition_arn"`
	JobDefinitionName string          `ns:"job_definition_name"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.Deployer.RemoteProvider = credsFactory("deployer")
}
