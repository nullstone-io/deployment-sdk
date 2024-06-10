package s3

import (
	"context"
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/cdn"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"reflect"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
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
	ctx = logging.ContextWithOsWriters(ctx, d.OsWriters)
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if len(d.Infra.CdnIds) < 1 {
		fmt.Fprintln(stdout)
		colorstring.Fprintln(stdout, "[bold]There are no attached CDNs. There is nothing to deploy.")
		return "", nil
	}

	cdnDeployer := cdn.Deployer{
		OsWriters: d.OsWriters,
		Details:   d.Details,
		Infra: cdn.Outputs{
			Region:               d.Infra.Region,
			Deployer:             d.Infra.Deployer,
			CdnIds:               d.Infra.CdnIds,
			ArtifactsKeyTemplate: d.Infra.ArtifactsKeyTemplate,
		},
		PostUpdateFn: d.updateEnvVars,
	}
	return cdnDeployer.Deploy(ctx, meta)
}

func (d Deployer) updateEnvVars(ctx context.Context, meta app.DeployMetadata) (bool, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	if d.Infra.EnvVarsFilename == "" {
		// If there is no env vars filename, there is nothing to update
		fmt.Fprintf(stdout, "The module for this application does not support environment variables. It is missing `env_vars_filename` output. Skipped updating environment variables s3 object.\n")
		return false, nil
	}

	original, err := GetEnvVars(ctx, d.Infra)
	if err != nil {
		return false, err
	}

	updated := map[string]string{}
	for k, v := range original {
		updated[k] = v
	}
	env_vars.UpdateStandard(updated, meta)
	if reflect.DeepEqual(original, updated) {
		return false, nil
	}
	fmt.Fprintf(stdout, "Updating environment variables s3 object %q\n", d.Infra.EnvVarsFilename)
	return true, PutEnvVars(ctx, d.Infra, updated)
}
