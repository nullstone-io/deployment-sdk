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
	Region       string            `ns:"region"`
	Deployer     nsaws.IamIdentity `ns:"deployer"`
	LambdaArn    string            `ns:"lambda_arn"`
	LambdaName   string            `ns:"lambda_name"`
	ImageRepoUrl docker.ImageUrl   `ns:"image_repo_url,optional"`
	// LambdaAlias is the alias that triggers use to invoke the function.
	// When set, a deploy only takes effect once this alias points at the newly published version.
	// When empty, triggers invoke the function directly ($LATEST).
	LambdaAlias string `ns:"lambda_alias,optional"`
	// LambdaVersionsToKeep is how many published versions remain after a deploy, counting the live one.
	// Only used when LambdaAlias is set. When the module does not emit it, DefaultVersionsToKeep is used.
	LambdaVersionsToKeep *int `ns:"lambda_versions_to_keep,optional"`
}

// DefaultVersionsToKeep is how many published versions remain after a deploy when the module does not say.
const DefaultVersionsToKeep = 3

// VersionsToKeep returns how many published versions to retain after a deploy.
// usingDefault is true when the module did not emit `lambda_versions_to_keep`.
// The live version is never pruned, so anything below 1 is treated as 1.
func (o Outputs) VersionsToKeep() (keep int, usingDefault bool) {
	if o.LambdaVersionsToKeep == nil {
		return DefaultVersionsToKeep, true
	}
	if *o.LambdaVersionsToKeep < 1 {
		return 1, false
	}
	return *o.LambdaVersionsToKeep, false
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	o.Deployer.RemoteProvider = credsFactory(types.AutomationPurposeDeploy, "deployer")
}

func (o Outputs) FunctionName() string {
	return o.LambdaName
}

func (o Outputs) DeployerAwsConfig() aws.Config {
	return nsaws.NewConfig(o.Deployer, o.Region)
}
