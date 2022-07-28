package beanstalk

import (
	"context"
	"fmt"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"time"
)

func NewDeployer(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Deployer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Deploy(ctx context.Context, version string) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if version == "" {
		return "", fmt.Errorf("--version is required to deploy app")
	}

	fmt.Fprintln(stdout, "Waiting for AWS to process application version...")
	for i := 0; i < 10; i++ {
		appVersion, err := GetApplicationVersion(ctx, d.Infra, version)
		if err != nil {
			return "", err
		} else if appVersion == nil {
			return "", fmt.Errorf("application version does not exist")
		}
		if appVersion.Status == ebtypes.ApplicationVersionStatusProcessed {
			break
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("cancelled")
		case <-time.After(2 * time.Second):
		}
	}

	fmt.Fprintf(stdout, "Updating application environment %q...\n", version)
	if err := UpdateEnvironment(ctx, d.Infra, version); err != nil {
		return "", fmt.Errorf("error updating application environment: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return version, nil
}
