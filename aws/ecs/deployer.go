package ecs

import (
	"context"
	"fmt"

	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
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
	colorstring.Fprintln(stdout, "[bold]Retrieved ECS service outputs")
	fmt.Fprintf(stdout, "	cluster_arn:    %s\n", d.Infra.ClusterArn())
	fmt.Fprintf(stdout, "	service_name:   %s\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "	image_repo_url: %s\n", d.Infra.ImageRepoUrl)
}

// Deploy takes the following steps to deploy an AWS ECS service
//
//	Get task definition
//	Change image tag in task definition
//	Register new task definition
//	Deregister old task definition
//	Update ECS Service (This always causes deployment)
func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, stderr := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	taskDef, err := GetTaskDefinition(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving current service information: %w", err)
	} else if taskDef == nil {
		return "", fmt.Errorf("could not find task definition")
	}

	taskDefTags, err := GetTaskDefinitionTags(ctx, d.Infra)
	if err != nil {
		// As we roll this out, the deployer user doesn't have permission to tag the task definition
		// If we cannot fetch them, leave tags nil so we don't try to update them
		fmt.Fprintln(stderr, "task definition tags will be cleared because of an error that occurred retrieving the existing tags:")
		fmt.Fprintln(stderr, err.Error())
	} else {
		taskDefTags = UpdateTaskDefTagVersion(taskDefTags, meta.Version)
	}

	updatedTaskDef, err := ReplaceTaskImageTag(d.Infra, *taskDef, meta.Version)
	if err != nil {
		return "", fmt.Errorf("error updating container version: %w", err)
	}
	fmt.Fprintln(stdout, fmt.Sprintf("Updating main container image tag to application version %q", meta.Version))
	updatedTaskDef = ReplaceEnvVars(*updatedTaskDef, meta)
	fmt.Fprintln(stdout, "Updating environment variables")
	if ReplaceOtelResourceAttributesEnvVar(updatedTaskDef, meta) {
		fmt.Fprintln(stdout, "Updating OpenTelemetry resource attributes (service.version and service.commit.sha)")
	}

	newTaskDef, err := UpdateTask(ctx, d.Infra, updatedTaskDef, taskDefTags, *taskDef.TaskDefinitionArn)
	if err != nil {
		return "", fmt.Errorf("error updating task with new image tag: %w", err)
	}
	fmt.Fprintln(stdout, "Updated task definition successfully")
	newTaskDefArn := *newTaskDef.TaskDefinitionArn

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No service name in app module. Skipping update service.\n")
		fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
		return "", nil
	}

	fmt.Fprintln(stdout, "Updating service with new task definition")
	deployment, err := UpdateServiceTask(ctx, d.Infra, newTaskDefArn)
	if err != nil {
		return "", fmt.Errorf("error deploying service: %w", err)
	} else if deployment == nil {
		fmt.Fprintf(stdout, "Updated service, but could not find a deployment.\n")
		return "", nil
	}
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return *deployment.Id, nil
}
