package cdn

import (
	"strings"

	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	Region               string            `ns:"region"`
	Deployer             nsaws.IamIdentity `ns:"deployer"`
	CdnIds               []string          `ns:"cdn_ids"`
	ArtifactsKeyTemplate string            `ns:"artifacts_key_template,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	o.Deployer.RemoteProvider = credsFactory(types.AutomationPurposeDeploy, "deployer")
}

func (o *Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}
