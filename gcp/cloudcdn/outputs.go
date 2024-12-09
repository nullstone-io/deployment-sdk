package cloudcdn

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"strings"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	ProjectId            string             `ns:"project_id"`
	Deployer             gcp.ServiceAccount `ns:"deployer"`
	CdnUrlMapNames       []string           `ns:"cdn_url_map_names"`
	ArtifactsKeyTemplate string             `ns:"artifacts_key_template,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.Uid, "deployer")
}

func (o *Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}
