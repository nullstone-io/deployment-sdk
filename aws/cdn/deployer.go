package cdn

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace)
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
	PostUpdateFn func(ctx context.Context, meta app.DeployMetadata) (bool, error)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	ctx = logging.ContextWithOsWriters(ctx, d.OsWriters)
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintf(stdout, "Updating CDN version to %q\n", meta.Version)
	changed, err := UpdateCdnVersion(ctx, d.Infra, meta.Version)
	if err != nil {
		return "", fmt.Errorf("error updating CDN version: %w", err)
	}

	if d.PostUpdateFn != nil {
		hasChanges, err := d.PostUpdateFn(ctx, meta)
		if err != nil {
			return "", err
		}
		changed = changed || hasChanges
	}

	// We only perform an invalidation if there were changes to the app
	if changed {
		fmt.Fprintln(stdout, "Invalidating cache in CDNs")
		invalidationIds, err := InvalidateCdnPaths(ctx, d.Infra, []string{"/*"})
		if err != nil {
			return "", fmt.Errorf("error invalidating /*: %w", err)
		}
		// NOTE: We only know how to return a single CDN invalidation ID
		//       The first iteration of the loop will return the first one
		for _, invalidationId := range invalidationIds {
			fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
			return invalidationId, nil
		}
	}
	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
