package aca

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v2"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/docker"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
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
	colorstring.Fprintln(stdout, "[bold]Retrieved Azure Container Apps outputs")
	fmt.Fprintf(stdout, "\tsubscription_id:    %s\n", d.Infra.SubscriptionId)
	fmt.Fprintf(stdout, "\tresource_group:     %s\n", d.Infra.ResourceGroup)
	fmt.Fprintf(stdout, "\tcontainer_app_name: %s\n", d.Infra.ContainerAppName)
	fmt.Fprintf(stdout, "\tjob_name:           %s\n", d.Infra.JobName)
	fmt.Fprintf(stdout, "\timage_repo_url:     %s\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if d.Infra.ContainerAppName != "" {
		return d.deployContainerApp(ctx, meta)
	} else if d.Infra.JobName != "" {
		return d.deployJob(ctx, meta)
	} else {
		fmt.Fprintf(stdout, "No container_app_name or job_name in app module. Skipping deployment.\n")
		return "", nil
	}
}

func (d Deployer) deployContainerApp(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	clientFactory, err := armappcontainers.NewClientFactory(d.Infra.SubscriptionId, &d.Infra.Deployer, nil)
	if err != nil {
		return "", fmt.Errorf("error creating ACA client: %w", err)
	}
	appsClient := clientFactory.NewContainerAppsClient()

	existing, err := appsClient.Get(ctx, d.Infra.ResourceGroup, d.Infra.ContainerAppName, nil)
	if err != nil {
		return "", fmt.Errorf("error retrieving container app %q: %w", d.Infra.ContainerAppName, err)
	}

	template := existing.Properties.Template
	mainIdx, mainContainer := getContainerByName(template.Containers, d.Infra.MainContainerName)
	if mainIdx < 0 {
		return "", fmt.Errorf("cannot find main container %q in template", d.Infra.MainContainerName)
	}

	setContainerImageTag(mainContainer, d.Infra.ImageRepoUrl, meta.Version)
	fmt.Fprintf(stdout, "Updating main container image tag to application version %q\n", meta.Version)
	replaceEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, "Updating environment variables")
	template.Containers[mainIdx] = mainContainer

	poller, err := appsClient.BeginUpdate(ctx, d.Infra.ResourceGroup, d.Infra.ContainerAppName, armappcontainers.ContainerApp{
		Location: existing.Location,
		Properties: &armappcontainers.ContainerAppProperties{
			Template: template,
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("error updating container app: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error waiting for container app update: %w", err)
	}

	revisionName := ""
	if result.Properties != nil && result.Properties.LatestRevisionName != nil {
		revisionName = *result.Properties.LatestRevisionName
	}

	fmt.Fprintf(stdout, "Updated container app successfully (revision: %s)\n", revisionName)
	return revisionName, nil
}

func (d Deployer) deployJob(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	clientFactory, err := armappcontainers.NewClientFactory(d.Infra.SubscriptionId, &d.Infra.Deployer, nil)
	if err != nil {
		return "", fmt.Errorf("error creating ACA client: %w", err)
	}
	jobsClient := clientFactory.NewJobsClient()

	existing, err := jobsClient.Get(ctx, d.Infra.ResourceGroup, d.Infra.JobName, nil)
	if err != nil {
		return "", fmt.Errorf("error retrieving job %q: %w", d.Infra.JobName, err)
	}

	template := existing.Properties.Template
	mainIdx, mainContainer := getJobContainerByName(template.Containers, d.Infra.MainContainerName)
	if mainIdx < 0 {
		return "", fmt.Errorf("cannot find main container %q in job template", d.Infra.MainContainerName)
	}

	setJobContainerImageTag(mainContainer, d.Infra.ImageRepoUrl, meta.Version)
	fmt.Fprintf(stdout, "Updating main container image tag to application version %q in job\n", meta.Version)
	replaceJobEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, "Updating environment variables in job")
	template.Containers[mainIdx] = mainContainer

	poller, err := jobsClient.BeginUpdate(ctx, d.Infra.ResourceGroup, d.Infra.JobName, armappcontainers.JobPatchProperties{
		Properties: &armappcontainers.JobPatchPropertiesProperties{
			Template: template,
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("error updating job: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error waiting for job update: %w", err)
	}

	fmt.Fprintln(stdout, "Updated job successfully")
	return d.Infra.JobName, nil
}

func getContainerByName(containers []*armappcontainers.Container, name string) (int, *armappcontainers.Container) {
	for i, c := range containers {
		if c.Name != nil && *c.Name == name {
			return i, c
		}
	}
	// If name is empty or not found, return the first container
	if len(containers) > 0 && name == "" {
		return 0, containers[0]
	}
	return -1, nil
}

func setContainerImageTag(container *armappcontainers.Container, existingImageUrl docker.ImageUrl, imageTag string) {
	if existingImageUrl.Repo == "" && container.Image != nil {
		existingImageUrl = docker.ParseImageUrl(*container.Image)
	}
	existingImageUrl.Digest = ""
	existingImageUrl.Tag = imageTag
	newImage := existingImageUrl.String()
	container.Image = &newImage
}

func replaceEnvVars(container *armappcontainers.Container, standard map[string]string) {
	if container.Env == nil {
		return
	}
	for i, ev := range container.Env {
		if ev.Name == nil {
			continue
		}
		if val, ok := standard[*ev.Name]; ok {
			// Only update env vars that are not secret refs
			if ev.SecretRef == nil || *ev.SecretRef == "" {
				container.Env[i].Value = &val
			}
		}
	}
}

func getJobContainerByName(containers []*armappcontainers.Container, name string) (int, *armappcontainers.Container) {
	return getContainerByName(containers, name)
}

func setJobContainerImageTag(container *armappcontainers.Container, existingImageUrl docker.ImageUrl, imageTag string) {
	setContainerImageTag(container, existingImageUrl, imageTag)
}

func replaceJobEnvVars(container *armappcontainers.Container, standard map[string]string) {
	replaceEnvVars(container, standard)
}
