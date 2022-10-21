package cdn

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
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	Infra        Outputs
	PostUpdateFn func(ctx context.Context, meta app.DeployMetadata) error
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Updating CDN version to %q\n", meta.Version)
	if err := UpdateCdnVersion(ctx, d.Infra, meta.Version); err != nil {
		return "", fmt.Errorf("error updating CDN version: %w", err)
	}

	if d.PostUpdateFn != nil {
		if err := d.PostUpdateFn(ctx, meta); err != nil {
			return "", err
		}
	}

	fmt.Fprintln(stdout, "Invalidating cache in CDNs")
	invalidationIds, err := InvalidateCdnPaths(ctx, d.Infra, []string{"/*"})
	if err != nil {
		return "", fmt.Errorf("error invalidating /*: %w", err)
	}
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)

	// NOTE: We only know how to return a single CDN invalidation ID
	//       The first iteration of the loop will return the first one
	for _, invalidationId := range invalidationIds {
		return invalidationId, nil
	}
	return "", nil
}
