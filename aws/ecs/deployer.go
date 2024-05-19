package ecs

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
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
	fmt.Fprintf(stdout, "ecs cluster: %q\n", d.Infra.ClusterArn())
	fmt.Fprintf(stdout, "ecs service: %q\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "repository image url: %q\n", d.Infra.ImageRepoUrl)
}

// Deploy takes the following steps to deploy an AWS ECS service
//
//	Get task definition
//	Change image tag in task definition
//	Register new task definition
//	Deregister old task definition
//	Update ECS Service (This always causes deployment)
func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	taskDef, err := GetTaskDefinition(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving current service information: %w", err)
	} else if taskDef == nil {
		return "", fmt.Errorf("could not find task definition")
	}

	updatedTaskDef, err := ReplaceTaskImageTag(d.Infra, *taskDef, meta.Version)
	if err != nil {
		return "", fmt.Errorf("error updating container version: %w", err)
	}
	updatedTaskDef = ReplaceEnvVars(*updatedTaskDef, meta)

	fmt.Fprintf(stdout, "Updating task definition version and environment variables\n")
	newTaskDef, err := UpdateTask(ctx, d.Infra, updatedTaskDef, *taskDef.TaskDefinitionArn)
	if err != nil {
		return "", fmt.Errorf("error updating task with new image tag: %w", err)
	}
	fmt.Fprintf(stdout, "Updated task definition version and environment variables\n")
	newTaskDefArn := *newTaskDef.TaskDefinitionArn

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No service name in app module. Skipping update service.\n")
		fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
		fmt.Fprintln(stdout, "")
		return "", nil
	}

	deployment, err := UpdateServiceTask(ctx, d.Infra, newTaskDefArn)
	if err != nil {
		return "", fmt.Errorf("error deploying service: %w", err)
	} else if deployment == nil {
		fmt.Fprintf(stdout, "Updated service, but could not find a deployment.\n")
		return "", nil
	}
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	fmt.Fprintln(stdout, "")
	return *deployment.Id, nil
}
