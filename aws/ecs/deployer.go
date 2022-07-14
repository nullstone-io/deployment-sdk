package ecs

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
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
		NsConfig:  nsConfig,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	NsConfig  api.Config
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "ecs cluster: %q\n", d.Infra.Cluster.ClusterArn)
	fmt.Fprintf(stdout, "ecs service: %q\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "repository image url: %q\n", d.Infra.ImageRepoUrl)
}

// Deploy takes the following steps to deploy an AWS ECS service
//   Get task definition
//   Change image tag in task definition
//   Register new task definition
//   Deregister old task definition
//   Update ECS Service (This always causes deployment)
func (d Deployer) Deploy(ctx context.Context, version string) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	taskDef, err := GetTaskDefinition(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving current service information: %w", err)
	} else if taskDef == nil {
		return "", fmt.Errorf("could not find task definition")
	}

	fmt.Fprintf(stdout, "Updating image tag to %q\n", version)
	newTaskDef, err := UpdateTaskImageTag(ctx, d.Infra, taskDef, version)
	if err != nil {
		return "", fmt.Errorf("error updating task with new image tag: %w", err)
	}
	newTaskDefArn := *newTaskDef.TaskDefinitionArn

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No service name in outputs. Skipping update service.\n")
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
	return *deployment.Id, nil
}
