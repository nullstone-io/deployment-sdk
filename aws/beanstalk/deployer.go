package beanstalk

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

func NewDeployer(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Deployer{
		OsWriters: osWriters,
		NsConfig:  nsConfig,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	NsConfig  api.Config
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Deploy(ctx context.Context, version string) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if version == "" {
		return "", fmt.Errorf("--version is required to deploy app")
	}

	fmt.Fprintf(stdout, "Updating application environment %q...\n", version)
	if err := UpdateEnvironment(ctx, d.Infra, version); err != nil {
		return "", fmt.Errorf("error updating application environment: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
