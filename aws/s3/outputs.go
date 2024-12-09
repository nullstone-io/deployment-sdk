package s3

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"strings"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	Region               string     `ns:"region"`
	Deployer             nsaws.User `ns:"deployer"`
	ArtifactsBucketName  string     `ns:"artifacts_bucket_name"`
	ArtifactsKeyTemplate string     `ns:"artifacts_key_template"`
	CdnIds               []string   `ns:"cdn_ids,optional"`
	EnvVarsFilename      string     `ns:"env_vars_filename,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.Deployer.RemoteProvider = credsFactory("deployer")
}

func (o *Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}
