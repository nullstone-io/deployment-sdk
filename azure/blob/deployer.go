package blob

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cdn/armcdn"
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

	return BlobDeployer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type BlobDeployer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d BlobDeployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved Azure Blob Storage outputs")
	fmt.Fprintf(stdout, "\tsubscription_id:  %s\n", d.Infra.SubscriptionId)
	fmt.Fprintf(stdout, "\tresource_group:   %s\n", d.Infra.ResourceGroup)
	fmt.Fprintf(stdout, "\tstorage_account:  %s\n", d.Infra.StorageAccount)
	fmt.Fprintf(stdout, "\tcontainer_name:   %s\n", d.Infra.ContainerName)
	fmt.Fprintf(stdout, "\tcdn_profile_name: %s\n", d.Infra.CdnProfileName)
	fmt.Fprintf(stdout, "\tcdn_endpoint:     %s\n", d.Infra.CdnEndpointName)
}

func (d BlobDeployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	if d.Infra.CdnProfileName == "" || d.Infra.CdnEndpointName == "" {
		fmt.Fprintln(stdout)
		colorstring.Fprintln(stdout, "[bold]There are no attached CDNs. There is nothing to deploy.")
		return "", nil
	}

	// Purge CDN cache to pick up the new content
	fmt.Fprintln(stdout, "Purging CDN cache...")
	cdnClient, err := armcdn.NewEndpointsClient(d.Infra.SubscriptionId, &d.Infra.Deployer, nil)
	if err != nil {
		return "", fmt.Errorf("error creating CDN client: %w", err)
	}

	contentPaths := []*string{strPtr("/*")}
	poller, err := cdnClient.BeginPurgeContent(ctx, d.Infra.ResourceGroup, d.Infra.CdnProfileName, d.Infra.CdnEndpointName, armcdn.PurgeParameters{
		ContentPaths: contentPaths,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("error purging CDN content: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error waiting for CDN purge: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed app %q successfully\n", d.Details.App.Name)
	return meta.Version, nil
}

func strPtr(s string) *string {
	return &s
}
