package s3

import (
	"context"
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
	if d.Infra.EnvVarsFilename == "" {
		// If there is no env vars filename, there is nothing to update
		return nil
	}
	envVars, err := GetEnvVars(ctx, d.Infra)
	if err != nil {
		return err
	}
	envVars = env_vars.UpdateStandard(envVars, meta)
	return PutEnVars(ctx, d.Infra, envVars)
}
