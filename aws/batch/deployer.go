package batch

import (
	"context"
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"strings"
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

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved Beanstalk outputs")
	fmt.Fprintf(stdout, "	region:              %s\n", d.Infra.Region)
	fmt.Fprintf(stdout, "	job_definition_name: %s\n", d.Infra.JobDefinitionName)
	fmt.Fprintf(stdout, "	image_repo_url:      %s\n", d.Infra.ImageRepoUrl)
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

	jobDef, allDefs, err := GetJobDefinition(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving current job information: %w", err)
	} else if jobDef == nil {
		return "", fmt.Errorf("could not find job definition")
	}
	fmt.Fprintf(stdout, "Current active job definition revision: %d\n", *jobDef.Revision)

	updatedJobDef := ReplaceJobDefinitionImageTag(d.Infra, *jobDef, meta.Version)
	updatedJobDef = ReplaceEnvVars(updatedJobDef, meta)

	fmt.Fprintf(stdout, "Updating job definition version and environment variables\n")
	newJobDefArn, revision, err := CreateJobDefinition(ctx, d.Infra, &updatedJobDef)
	if err != nil {
		return "", fmt.Errorf("error updating job definition with new image tag: %w", err)
	}
	if newJobDefArn == nil {
		return "", fmt.Errorf("new job definition arn is nil")
	}
	if revision == nil {
		return "", fmt.Errorf("new job definition revision is nil")
	}
	fmt.Fprintf(stdout, "New job definition created: (arn - %s, revision - %d)\n", *newJobDefArn, *revision)

	deregisteredRevisions, err := DeregisterJobDefinitions(ctx, d.Infra, allDefs)
	deregistered := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(deregisteredRevisions)), ", "), "[]")
	fmt.Fprintf(stdout, "The following revisions have been deregistered: %s\n", deregistered)
	if err != nil {
		return "", fmt.Errorf("error deregistering old job definitions: %w", err)
	}
	fmt.Fprintln(stdout, "Current active job definition has been successfully updated")
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "No service name in app module. Skipping update service.\n")
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
