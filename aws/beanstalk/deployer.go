package beanstalk

import (
	"context"
	"fmt"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"time"
)

func NewDeployer(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
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

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if meta.Version == "" {
		return "", fmt.Errorf("--version is required to deploy app")
	}

	fmt.Fprintln(stdout, "Waiting for AWS to process application version...")
	for i := 0; i < 10; i++ {
		appVersion, err := GetApplicationVersion(ctx, d.Infra, meta.Version)
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

	fmt.Fprintf(stdout, "Updating application environment %q...\n", meta.Version)
	if err := UpdateEnvironment(ctx, d.Infra, meta.Version); err != nil {
		return "", fmt.Errorf("error updating application environment: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return meta.Version, nil
}
