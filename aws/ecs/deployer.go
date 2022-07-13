package ecs

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
)

type Outputs struct {
	Region            string          `ns:"region"`
	ServiceName       string          `ns:"service_name"`
	TaskArn           string          `ns:"task_arn"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher       nsaws.User      `ns:"image_pusher,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`

	Cluster ClusterOutputs `ns:",connectionContract=cluster/aws/ecs:*"`
}

type ClusterOutputs struct {
	ClusterArn string `ns:"cluster_arn"`
}

func NewDeployer(logger *log.Logger, nsConfig api.Config, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Deployer{
		Logger:   logger,
		NsConfig: nsConfig,
		Details:  appDetails,
		Infra:    outs,
	}, nil
}

type Deployer struct {
	Logger   *log.Logger
	NsConfig api.Config
	Details  app.Details
	Infra    Outputs
}

func (d Deployer) Print() {
	d.Logger.Printf("ecs cluster: %q\n", d.Infra.Cluster.ClusterArn)
	d.Logger.Printf("ecs service: %q\n", d.Infra.ServiceName)
	d.Logger.Printf("repository image url: %q\n", d.Infra.ImageRepoUrl)
}

// Deploy takes the following steps to deploy an AWS ECS service
//   Get task definition
//   Change image tag in task definition
//   Register new task definition
//   Deregister old task definition
//   Update ECS Service (This always causes deployment)
func (d Deployer) Deploy(ctx context.Context, version string) (*string, error) {
	d.Print()

	if version == "" {
		return nil, fmt.Errorf("no version specified, version is required to deploy")
	}

	d.Logger.Printf("Deploying app %q\n", d.Details.App.Name)

	taskDef, err := GetTaskDefinition(ctx, d.Infra)
	if err != nil {
		return nil, fmt.Errorf("error retrieving current service information: %w", err)
	} else if taskDef == nil {
		return nil, fmt.Errorf("could not find task definition")
	}

	d.Logger.Printf("Updating image tag to %q\n", version)
	newTaskDef, err := UpdateTaskImageTag(ctx, d.Infra, taskDef, version)
	if err != nil {
		return nil, fmt.Errorf("error updating task with new image tag: %w", err)
	}
	newTaskDefArn := *newTaskDef.TaskDefinitionArn

	if d.Infra.ServiceName == "" {
		d.Logger.Printf("No service name in outputs. Skipping update service.")
		return nil, nil
	}

	deployment, err := UpdateServiceTask(ctx, d.Infra, newTaskDefArn)
	if err != nil {
		return nil, fmt.Errorf("error deploying service: %w", err)
	}
	d.Logger.Printf("Deployed app %q\n", d.Details.App.Name)
	return deployment.Id, nil
}
