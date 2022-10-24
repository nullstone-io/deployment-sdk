package lambda_zip

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/nullstone-io/deployment-sdk/aws"
	"strings"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	Region               string     `ns:"region"`
	Deployer             nsaws.User `ns:"deployer"`
	LambdaName           string     `ns:"lambda_name"`
	ArtifactsBucketName  string     `ns:"artifacts_bucket_name"`
	ArtifactsKeyTemplate string     `ns:"artifacts_key_template"`
}

func (o Outputs) FunctionName() string {
	return o.LambdaName
}

func (o Outputs) ArtifactsKey(appVersion string) string {
	tmpl := o.ArtifactsKeyTemplate
	if tmpl == "" {
		tmpl = "{{app-version}}"
	}
	return strings.Replace(tmpl, KeyTemplateAppVersion, appVersion, -1)
}

func (o Outputs) DeployerAwsConfig() aws.Config {
	return nsaws.NewConfig(o.Deployer, o.Region)
}
