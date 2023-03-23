package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DeployReferenceLatest = "latest-revision"
	DeployReferenceNoop   = "no-updated-revision"
)

func NewDeployer(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
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
	fmt.Fprintf(stdout, "gke cluster: %q\n", d.Infra.Cluster.ClusterId)
	fmt.Fprintf(stdout, "gke service: %q\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "repository image url: %q\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, stderr := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No service name in app module. Skipping update service.\n")
		fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
		fmt.Fprintln(stdout, "")
		return "", nil
	}

	kubeClient, err := CreateKubeClient(ctx, d.Infra.Cluster.Deployer, d.Infra.Cluster)
	if err != nil {
		return "", err
	}

	deployment, err := kubeClient.AppsV1().Deployments(d.Infra.ServiceNamespace).Get(ctx, d.Infra.ServiceName, meta_v1.GetOptions{})
	if err != nil {
		return "", err
	}

	podSpec := deployment.Spec.Template.Spec
	if podSpec, err = k8s.SetContainerImageTag(podSpec, d.Infra.MainContainerName, meta.Version); err != nil {
		return "", fmt.Errorf("error updating pod spec with new image tag: %w", err)
	}
	std := env_vars.GetStandard(meta)
	podSpec = k8s.ReplaceEnvVars(podSpec, std)

	deployment.Spec.Template.Spec = podSpec
	updated, err := kubeClient.AppsV1().Deployments(d.Infra.ServiceNamespace).Update(ctx, deployment, meta_v1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("error deploying app: %w", err)
	}

	curRevision, err := k8s.Revision(deployment)
	if err != nil {
		curRevision = 1
	}

	revision := ""
	if updatedRevision, err := k8s.Revision(updated); err != nil {
		revision = "(latest)"
		fmt.Fprintln(stderr, "Unable to find deployment revision. Relying on latest deployment revision to track rollout.")
	} else if updatedRevision == curRevision {
		revision = "(noop)"
		fmt.Fprintln(stdout, "No changes made to deployment.")
	} else {
		revision = fmt.Sprintf("%d", updatedRevision)
		fmt.Fprintf(stdout, "Created new deployment revision %s.\n", revision)
	}

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	fmt.Fprintln(stdout, "")
	return revision, nil
}
