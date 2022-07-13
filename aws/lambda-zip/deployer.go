package lambda_zip

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
	"strings"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	Region               string     `ns:"region"`
	Deployer             nsaws.User `ns:"deployer"`
	LambdaArn            string     `ns:"lambda_arn"`
	LambdaName           string     `ns:"lambda_name"`
	ArtifactsBucketName  string     `ns:"artifacts_bucket_name"`
	ArtifactsKeyTemplate string     `ns:"artifacts_key_template"`
}

func (o Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}

func NewDeployer(logger *log.Logger, nsConfig api.Config, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Deployer{
		Logger:   logger,
		NsConfig: nsConfig,
		Details:  appDetails,
		Infra:    outs,
	}, nil
}

type Deployer struct {
	Logger   *log.Logger
	NsConfig api.Config
	Details  app.Details
	Infra    Outputs
}

func (d Deployer) Deploy(ctx context.Context, version string) (*string, error) {
	d.Logger.Printf("Deploying app %q\n", d.Details.App.Name)
	if version == "" {
		return nil, fmt.Errorf("--version is required to deploy app")
	}

	d.Logger.Printf("Updating lambda to %q\n", version)
	if err := UpdateLambdaVersion(ctx, d.Infra, version); err != nil {
		return nil, fmt.Errorf("error updating lambda version: %w", err)
	}

	d.Logger.Printf("Deployed app %q\n", d.Details.App.Name)
	return nil, nil
}
