package lambda_container

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region       string          `ns:"region"`
	Deployer     nsaws.User      `ns:"deployer"`
	LambdaArn    string          `ns:"lambda_arn"`
	LambdaName   string          `ns:"lambda_name"`
	ImageRepoUrl docker.ImageUrl `ns:"image_repo_url,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.Deployer.RemoteProvider = credsFactory("deployer")
}

func (o Outputs) FunctionName() string {
	return o.LambdaName
}

func (o Outputs) DeployerAwsConfig() aws.Config {
	return nsaws.NewConfig(o.Deployer, o.Region)
}
