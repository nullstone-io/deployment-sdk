package k8s

import (
	"context"
	"errors"
	"fmt"

	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Deployer struct {
	K8sNamespace      string
	AppName           string
	MainContainerName string
	ServiceName       string
	JobDefinitionName string
	OsWriters         logging.OsWriters
}

func (d Deployer) Validate(meta app.DeployMetadata) (bool, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if meta.Version == "" {
		return false, fmt.Errorf("no version specified, version is required to deploy")
	}

	if d.ServiceName == "" && d.JobDefinitionName == "" {
		fmt.Fprintf(stdout, "No service_name or job_definition_name in app module. Skipping update.\n")
		return false, nil
	}
	return true, nil
}

func (d Deployer) Deploy(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) (string, error) {
	if valid, err := d.Validate(meta); !valid {
		return "", err
	}

	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.AppName)

	var reference string
	var err error
	if d.ServiceName != "" {
		reference, err = d.deployService(ctx, kubeClient, meta)
	} else if d.JobDefinitionName != "" {
		err = d.deployJob(ctx, kubeClient, meta)
	}
	if err != nil {
		return "", err
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.AppName)
	fmt.Fprintln(stdout, "")
	return reference, nil
}

func (d Deployer) deployService(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	deployment, err := kubeClient.AppsV1().Deployments(d.K8sNamespace).Get(ctx, d.ServiceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	curGeneration := deployment.Generation

	// Update deployment definition
	deployment.ObjectMeta = UpdateVersionLabel(deployment.ObjectMeta, meta.Version)
	deployment.Spec.Template, err = d.updatePodTemplate(deployment.Spec.Template, "service", meta)
	if err != nil {
		return "", err
	}

	updated, err := kubeClient.AppsV1().Deployments(d.K8sNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
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

	return reference, nil
}

func (d Deployer) deployJob(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) error {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if err := d.updateJobTemplateConfig(ctx, kubeClient, meta); err != nil {
		return fmt.Errorf("error updating job template: %w", err)
	}
	fmt.Fprintln(stdout, "Updated job template successfully")

	if err := d.updateCronJobs(ctx, kubeClient, meta); err != nil {
		return fmt.Errorf("error updating cron jobs: %w", err)
	}

	return nil
}

// updateJobTemplateConfig updates the job definition that is stored as a ConfigMap
func (d Deployer) updateJobTemplateConfig(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) error {
	// Retrieve and update job definition
	jobDef, configMap, err := GetJobDefinition(ctx, kubeClient, d.K8sNamespace, d.JobDefinitionName)
	if err != nil {
		return err
	}
	jobDef.ObjectMeta = UpdateVersionLabel(jobDef.ObjectMeta, meta.Version)
	jobDef.Spec.Template, err = d.updatePodTemplate(jobDef.Spec.Template, "job definition", meta)
	if err != nil {
		return fmt.Errorf("cannot find main container %q in spec", d.MainContainerName)
	}
	if err := UpdateJobDefinition(ctx, kubeClient, d.K8sNamespace, jobDef, configMap); err != nil {
		return err
	}
	return nil
}

// updateCronJobs updates each batch/v1/CronJob configured on this app
func (d Deployer) updateCronJobs(ctx context.Context, kubeClient *kubernetes.Clientset, meta app.DeployMetadata) error {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	appLabel := fmt.Sprintf("nullstone.io/app=%s", d.AppName)
	jobs, err := kubeClient.BatchV1().CronJobs(d.K8sNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return fmt.Errorf("error retrieving CronJobs: %w", err)
	}

	if len(jobs.Items) < 1 {
		return nil
	}

	var errs []error
	for _, job := range jobs.Items {
		job.ObjectMeta = UpdateVersionLabel(job.ObjectMeta, meta.Version)
		job.Spec.JobTemplate.Spec.Template, err = d.updatePodTemplate(job.Spec.JobTemplate.Spec.Template, "cron job", meta)
		if err != nil {
			errs = append(errs, fmt.Errorf("error modifying cron job spec %q: %w", job.Name, err))
			continue
		}
		if _, err := kubeClient.BatchV1().CronJobs(d.K8sNamespace).Update(ctx, &job, metav1.UpdateOptions{}); err != nil {
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

func (d Deployer) updatePodTemplate(template corev1.PodTemplateSpec, appType string, meta app.DeployMetadata) (corev1.PodTemplateSpec, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	template.ObjectMeta = UpdateVersionLabel(template.ObjectMeta, meta.Version)
	mainContainerIndex, mainContainer := GetContainerByName(template, d.MainContainerName)
	if mainContainerIndex < 0 {
		return template, fmt.Errorf("cannot find main container %q in spec", d.MainContainerName)
	}
	SetContainerImageTag(mainContainer, meta.Version)
	fmt.Fprintln(stdout, fmt.Sprintf("Updating main container image tag to application version %q in %s", meta.Version, appType))
	ReplaceEnvVars(mainContainer, env_vars.GetStandard(meta))
	fmt.Fprintln(stdout, fmt.Sprintf("Updating environment variables in %s", appType))
	if ReplaceOtelResourceAttributesEnvVar(mainContainer, meta.Version, meta.CommitSha) {
		fmt.Fprintln(stdout, fmt.Sprintf("Updating OpenTelemetry resource attributes (service.version and service.commit.sha) in %s", appType))
	}
	template.Spec.Containers[mainContainerIndex] = *mainContainer
	return template, nil
}
