package eks

import (
	"context"
	"fmt"

	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
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
	colorstring.Fprintln(stdout, "[bold]Retrieved EKS service outputs")
	fmt.Fprintf(stdout, "	cluster_endpoint:    %s\n", d.Infra.ClusterNamespace.ClusterEndpoint)
	fmt.Fprintf(stdout, "	service_namespace:   %s\n", d.Infra.ServiceNamespace)
	fmt.Fprintf(stdout, "	service_name:        %s\n", d.Infra.ServiceName)
	fmt.Fprintf(stdout, "	job_definition_name: %s\n", d.Infra.JobDefinitionName)
	fmt.Fprintf(stdout, "	image_repo_url:      %s\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	d.Print()

	deployer := k8s.Deployer{
		K8sNamespace:      d.Infra.ServiceNamespace,
		AppName:           d.Details.App.Name,
		MainContainerName: d.Infra.MainContainerName,
		ServiceName:       d.Infra.ServiceName,
		JobDefinitionName: d.Infra.JobDefinitionName,
		OsWriters:         d.OsWriters,
	}
	if valid, err := deployer.Validate(meta); !valid {
		return "", err
	}
	kubeClient, err := CreateKubeClient(ctx, d.Infra.ClusterNamespace, d.Infra.Deployer)
	if err != nil {
		return "", fmt.Errorf("error creating kubernetes client: %w", err)
	}

	return deployer.Deploy(ctx, kubeClient, meta)
}
