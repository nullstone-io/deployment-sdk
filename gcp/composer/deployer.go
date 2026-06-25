package composer

import (
	"context"
	"fmt"
	"maps"

	"cloud.google.com/go/orchestration/airflow/service/apiv1/servicepb"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

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

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved Cloud Composer environment outputs")
	fmt.Fprintf(stdout, "\tenvironment_name: %s\n", d.Infra.EnvironmentName)
}

// Deploy refreshes the standard env variables (NULLSTONE_VERSION, NULLSTONE_COMMIT_SHA, OTEL resource
// attributes) on the Composer environment's software config to reflect the deployed application version.
//
// Following the deployment-sdk convention, only env variables that already exist are updated. The full
// set of env variables is owned by Terraform, so we avoid adding/removing keys to prevent thrashing
// between code deploys and IaC runs.
func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	client, err := NewEnvironmentsClient(ctx, d.Infra.Deployer)
	if err != nil {
		return "", fmt.Errorf("error creating Composer client: %w", err)
	}
	defer client.Close()

	name := environmentResourceName(d.Infra)
	env, err := client.GetEnvironment(ctx, &servicepb.GetEnvironmentRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("error getting Composer environment: %w", err)
	}

	original := env.GetConfig().GetSoftwareConfig().GetEnvVariables()
	updated := maps.Clone(original)
	env_vars.UpdateStandard(updated, meta)
	if u, changed := env_vars.ReplaceOtelResourceAttributes(updated, meta, false); changed {
		updated = u
	}

	if maps.Equal(original, updated) {
		fmt.Fprintln(stdout, "No environment variable changes to deploy.")
		return "", nil
	}

	fmt.Fprintf(stdout, "Updating environment variables on Composer environment %q...\n", d.Infra.EnvironmentName)
	op, err := client.UpdateEnvironment(ctx, &servicepb.UpdateEnvironmentRequest{
		Name: name,
		Environment: &servicepb.Environment{
			Config: &servicepb.EnvironmentConfig{
				SoftwareConfig: &servicepb.SoftwareConfig{EnvVariables: updated},
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"config.softwareConfig.envVariables"}},
	})
	if err != nil {
		return "", fmt.Errorf("error updating Composer environment: %w", err)
	}
	return op.Name(), nil
}
