package cloudrun

import (
	"context"
	"fmt"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/docker"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/otel"
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
	colorstring.Fprintln(stdout, "[bold]Retrieved Cloud Run service outputs")
	fmt.Fprintf(stdout, "\tservice_name:   %s\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "\tjob_id:         %s\n", d.Infra.JobId)
	fmt.Fprintf(stdout, "\timage_repo_url: %s\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if d.Infra.ServiceName != "" {
		return d.deployService(ctx, meta)
	} else if d.Infra.JobId != "" {
		return d.deployJob(ctx, meta)
	} else {
		fmt.Fprintf(stdout, "No service_name or job_name in app module. Skipping deployment.\n")
		return "", nil
	}
}

func (d Deployer) deployService(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	client, err := NewServicesClient(ctx, d.Infra.Deployer)
	if err != nil {
		return "", err
	}

	svc, err := client.GetService(ctx, &runpb.GetServiceRequest{Name: d.Infra.ServiceName})
	if err != nil {
		return "", err
	} else if svc == nil {
		return "", fmt.Errorf("cloud run service %q not found", d.Infra.ServiceName)
	}

	mainContainerIndex, mainContainer := GetContainerByName(svc.Template.Containers, d.Infra.MainContainerName)
	if mainContainerIndex < 0 {
		return "", fmt.Errorf("cannot find main container %q in template", d.Infra.MainContainerName)
	}
	SetContainerImageTag(mainContainer, d.Infra.ImageRepoUrl, meta.Version)
	fmt.Fprintln(stdout, fmt.Sprintf("Updating main container image tag to application version %q in service", meta.Version))
	ReplaceEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, "Updating environment variables in service")
	if ReplaceOtelResourceAttributesEnvVar(mainContainer, meta) {
		fmt.Fprintln(d.OsWriters.Stdout(), "Updating OpenTelemetry resource attributes (service.version and service.commit.sha) in service")
	}
	svc.Template.Containers[mainContainerIndex] = mainContainer

	op, err := client.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: svc})
	if err != nil {
		return "", err
	}
	fmt.Fprintln(stdout, "Updated service successfully")
	return op.Name(), nil
}

func (d Deployer) deployJob(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	client, err := NewJobsClient(ctx, d.Infra.Deployer)
	if err != nil {
		return "", err
	}

	job, err := client.GetJob(ctx, &runpb.GetJobRequest{Name: d.Infra.JobId})
	if err != nil {
		return "", fmt.Errorf("error retrieving job definition: %w", err)
	} else if job == nil {
		return "", fmt.Errorf("cloud run job %q not found", d.Infra.JobId)
	}

	mainContainerIndex, mainContainer := GetContainerByName(job.Template.Template.Containers, d.Infra.MainContainerName)
	if mainContainerIndex < 0 {
		return "", fmt.Errorf("cannot find main container %q in template", d.Infra.MainContainerName)
	}
	SetContainerImageTag(mainContainer, d.Infra.ImageRepoUrl, meta.Version)
	fmt.Fprintln(stdout, fmt.Sprintf("Updating main container image tag to application version %q in job", meta.Version))
	ReplaceEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, "Updating environment variables in job")
	if ReplaceOtelResourceAttributesEnvVar(mainContainer, meta) {
		fmt.Fprintln(d.OsWriters.Stdout(), "Updating OpenTelemetry resource attributes (service.version and service.commit.sha) in job")
	}
	job.Template.Template.Containers[mainContainerIndex] = mainContainer

	op, err := client.UpdateJob(ctx, &runpb.UpdateJobRequest{Job: job})
	if err != nil {
		return "", fmt.Errorf("error updating job definition: %w", err)
	}
	fmt.Fprintln(stdout, "Updated job successfully")
	return op.Name(), nil
}

func GetContainerByName(containers []*runpb.Container, name string) (int, *runpb.Container) {
	for i, container := range containers {
		if container.Name == name {
			return i, container
		}
	}
	return -1, nil
}

func SetContainerImageTag(container *runpb.Container, existingImageUrl docker.ImageUrl, imageTag string) {
	if existingImageUrl.Repo == "" {
		existingImageUrl = docker.ParseImageUrl(container.Image)
	}
	existingImageUrl.Digest = ""
	existingImageUrl.Tag = imageTag
	container.Image = existingImageUrl.String()
}

func ReplaceEnvVars(container *runpb.Container, standard map[string]string) {
	for i, cur := range container.Env {
		if val, ok := standard[cur.Name]; ok {
			ReplaceEnvVarValue(container.Env[i], func(previous string) string { return val })
		}
	}
}

func ReplaceOtelResourceAttributesEnvVar(container *runpb.Container, meta app.DeployMetadata) bool {
	for i, cur := range container.Env {
		if cur.Name == otel.ResourceAttributesEnvName {
			ReplaceEnvVarValue(container.Env[i], otel.UpdateResourceAttributes(meta.Version, meta.CommitSha))
			return true
		}
	}
	return false
}

func ReplaceEnvVarValue(cur *runpb.EnvVar, fn func(previous string) string) {
	// We only change env vars that are not secret refs
	if vs := cur.GetValueSource(); vs == nil || vs.SecretKeyRef == nil {
		cur.Values = &runpb.EnvVar_Value{Value: fn(cur.GetValue())}
	}
}
