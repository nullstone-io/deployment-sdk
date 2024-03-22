package batch

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace)
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

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "job definition: %q\n", d.Infra.JobDefinitionArn)
	fmt.Fprintf(stdout, "repository image url: %q\n", d.Infra.ImageRepoUrl)
}

// Deploy takes the following steps to deploy an AWS ECS service
//
//	Get job definition
//	Change image tag in job definition
//	Register new job definition
//	Deregister old job definition
func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	jobDef, err := GetJobDefinition(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving current job information: %w", err)
	} else if jobDef == nil {
		return "", fmt.Errorf("could not find job definition")
	}

	updatedJobDef := ReplaceJobDefinitionImageTag(d.Infra, *jobDef, meta.Version)
	updatedJobDef = ReplaceEnvVars(updatedJobDef, meta)

	fmt.Fprintf(stdout, "Updating job definition version and environment variables\n")
	_, err = UpdateJobDefinition(ctx, d.Infra, &updatedJobDef, *jobDef.JobDefinitionArn)
	if err != nil {
		return "", fmt.Errorf("error updating job definition with new image tag: %w", err)
	}
	fmt.Fprintf(stdout, "Updated job definition version and environment variables\n")

	fmt.Fprintf(stdout, "No service name in app module. Skipping update service.\n")
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	fmt.Fprintln(stdout, "")
	return "", nil
}
