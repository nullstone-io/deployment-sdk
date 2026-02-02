package gke

import (
	"context"
	"errors"
	"fmt"

	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DeployReferenceNoop = "no-updated-revision"
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
	colorstring.Fprintln(stdout, "[bold]Retrieved GKE service outputs")
	fmt.Fprintf(stdout, "	cluster_endpoint:    %s\n", d.Infra.ClusterNamespace.ClusterEndpoint)
	fmt.Fprintf(stdout, "	service_namespace:   %s\n", d.Infra.ServiceNamespace)
	fmt.Fprintf(stdout, "	service_name:        %s\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "	job_definition_name: %s\n", d.Infra.JobDefinitionName)
	fmt.Fprintf(stdout, "	image_repo_url:      %s\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if d.Infra.ServiceName == "" {
		if d.Infra.JobDefinitionName == "" {
			fmt.Fprintf(stdout, "No service_name or job_definition_name in app module. Skipping update service.\n")
			fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
			fmt.Fprintln(stdout, "")
			return "", nil
		}

		return d.deployJobTemplate(ctx, meta)
	}
	return d.deployService(ctx, meta)
}

func (d Deployer) deployService(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	kubeClient, err := CreateKubeClient(ctx, d.Infra.ClusterNamespace, d.Infra.Deployer)
	if err != nil {
		return "", err
	}

	deployment, err := kubeClient.AppsV1().Deployments(d.Infra.ServiceNamespace).Get(ctx, d.Infra.ServiceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	curGeneration := deployment.Generation

	// Update deployment definition
	deployment.ObjectMeta = k8s.UpdateVersionLabel(deployment.ObjectMeta, meta.Version)
	deployment.Spec.Template, err = d.updatePodTemplate(deployment.Spec.Template, "service", meta)
	if err != nil {
		return "", err
	}

	updated, err := kubeClient.AppsV1().Deployments(d.Infra.ServiceNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("error deploying app: %w", err)
	}
	fmt.Fprintln(stdout, "Updated deployment successfully")
	updGeneration := updated.Generation
	reference := fmt.Sprintf("%d", updGeneration)

	if curGeneration == updGeneration {
		reference = DeployReferenceNoop
		fmt.Fprintln(stdout, "No changes made to deployment.")
	} else {
		fmt.Fprintf(stdout, "Created new deployment (generation = %s).\n", reference)
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return reference, nil
}

func (d Deployer) deployJobTemplate(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	kubeClient, err := CreateKubeClient(ctx, d.Infra.ClusterNamespace, d.Infra.Deployer)
	if err != nil {
		return "", err
	}

	if err := d.updateJobTemplateConfig(ctx, kubeClient, meta); err != nil {
		return "", fmt.Errorf("error updating job template: %w", err)
	}
	fmt.Fprintln(stdout, "Updated job template successfully")

	if err := d.updateCronJobs(ctx, kubeClient, meta); err != nil {
		return "", fmt.Errorf("error updating cron jobs: %w", err)
	}

	return "", nil
}

// updateJobTemplateConfig updates the job definition that is stored as a ConfigMap
func (d Deployer) updateJobTemplateConfig(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) error {
	// Retrieve and update job definition
	jobDef, configMap, err := k8s.GetJobDefinition(ctx, kubeClient, d.Infra.ServiceNamespace, d.Infra.JobDefinitionName)
	if err != nil {
		return err
	}
	jobDef.ObjectMeta = k8s.UpdateVersionLabel(jobDef.ObjectMeta, meta.Version)
	jobDef.Spec.Template, err = d.updatePodTemplate(jobDef.Spec.Template, "job definition", meta)
	if err != nil {
		return fmt.Errorf("cannot find main container %q in spec", d.Infra.MainContainerName)
	}
	if err := k8s.UpdateJobDefinition(ctx, kubeClient, d.Infra.ServiceNamespace, jobDef, configMap); err != nil {
		return err
	}
	return nil
}

// updateCronJobs updates each batch/v1/CronJob configured on this app
func (d Deployer) updateCronJobs(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) error {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	appLabel := fmt.Sprintf("nullstone.io/app=%s", d.Infra.ServiceName)
	jobs, err := kubeClient.BatchV1().CronJobs(d.Infra.ServiceNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return fmt.Errorf("error retrieving CronJobs: %w", err)
	}

	if len(jobs.Items) < 1 {
		return nil
	}

	var errs []error
	for _, job := range jobs.Items {
		job.ObjectMeta = k8s.UpdateVersionLabel(job.ObjectMeta, meta.Version)
		job.Spec.JobTemplate.Spec.Template, err = d.updatePodTemplate(job.Spec.JobTemplate.Spec.Template, "cron job", meta)
		if err != nil {
			errs = append(errs, fmt.Errorf("error modifying cron job spec %q: %w", job.Name, err))
			continue
		}
		if _, err := kubeClient.BatchV1().CronJobs(d.Infra.ServiceNamespace).Update(ctx, &job, metav1.UpdateOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("error updating cron job %q: %w", job.Name, err))
			continue
		}
		fmt.Fprintf(stdout, "Updated cron job %q successfully\n", job.Name)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (d Deployer) updatePodTemplate(template v1.PodTemplateSpec, appType string, meta app.DeployMetadata) (v1.PodTemplateSpec, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	template.ObjectMeta = k8s.UpdateVersionLabel(template.ObjectMeta, meta.Version)
	mainContainerIndex, mainContainer := k8s.GetContainerByName(template, d.Infra.MainContainerName)
	if mainContainerIndex < 0 {
		return template, fmt.Errorf("cannot find main container %q in spec", d.Infra.MainContainerName)
	}
	k8s.SetContainerImageTag(mainContainer, meta.Version)
	fmt.Fprintln(stdout, fmt.Sprintf("Updating main container image tag to application version %q in %s", meta.Version, appType))
	k8s.ReplaceEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, fmt.Sprintf("Updating environment variables in %s", appType))
	if k8s.ReplaceOtelResourceAttributesEnvVar(mainContainer, meta.Version, meta.CommitSha) {
		fmt.Fprintln(d.OsWriters.Stdout(), fmt.Sprintf("Updating OpenTelemetry resource attributes (service.version and service.commit.sha) in %s", appType))
	}
	template.Spec.Containers[mainContainerIndex] = *mainContainer
	return template, nil
}
