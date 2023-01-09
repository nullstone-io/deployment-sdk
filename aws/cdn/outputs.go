package cdn

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"strings"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	Region               string     `ns:"region"`
	Deployer             nsaws.User `ns:"deployer"`
	CdnIds               []string   `ns:"cdn_ids"`
	ArtifactsKeyTemplate string     `ns:"artifacts_key_template,optional"`
}

func (o Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}
