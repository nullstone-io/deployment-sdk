package s3

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
)

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
		return nil, fmt.Errorf("no version specified, version is required to deploy")
	}

	d.Logger.Printf("Updating CDN version to %q\n", version)
	if err := UpdateCdnVersion(ctx, d.Infra, version); err != nil {
		return nil, fmt.Errorf("error updating CDN version: %w", err)
	}

	d.Logger.Println("Invalidating cache in CDNs")
	if err := InvalidateCdnPaths(ctx, d.Infra, []string{"/*"}); err != nil {
		return nil, fmt.Errorf("error invalidating /*: %w", err)
	}

	d.Logger.Printf("Deployed app %q\n", d.Details.App.Name)
	return nil, nil
}
