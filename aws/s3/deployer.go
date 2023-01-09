package s3

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/cdn"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
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

	if len(d.Infra.CdnIds) < 1 {
		fmt.Fprintf(stdout, "There are no attached CDNs. There is nothing to deploy.\n")
		return "", nil
	}

	cdnDeployer := cdn.Deployer{
		OsWriters: d.OsWriters,
		Details:   d.Details,
		Infra: cdn.Outputs{
			Region:   d.Infra.Region,
			Deployer: d.Infra.Deployer,
			CdnIds:   d.Infra.CdnIds,
		},
		PostUpdateFn: d.updateEnvVars,
	}
	return cdnDeployer.Deploy(ctx, meta)
}

func (d Deployer) updateEnvVars(ctx context.Context, meta app.DeployMetadata) error {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	if d.Infra.EnvVarsFilename == "" {
		// If there is no env vars filename, there is nothing to update
		fmt.Fprintf(stdout, "The module for this application does not support environment variables. It is missing `env_vars_filename` output. Skipped updating environment variables s3 object.\n")
		return nil
	}

	fmt.Fprintf(stdout, "Updating environment variables s3 object %q\n", d.Infra.EnvVarsFilename)
	envVars, err := GetEnvVars(ctx, d.Infra)
	if err != nil {
		return err
	}
	env_vars.UpdateStandard(envVars, meta)
	if err := PutEnvVars(ctx, d.Infra, envVars); err != nil {
		return err
	}
	return nil
}
