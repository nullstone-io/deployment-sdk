package blob

import (
	"context"
	"fmt"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployStatusGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &BlobDeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type BlobDeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d *BlobDeployStatusGetter) Close() {}

func (d *BlobDeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	stdout := d.OsWriters.Stdout()

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	// For blob/CDN deployments, the deploy call (CDN purge) is synchronous via PollUntilDone.
	// By the time we get here, the deployment is already complete.
	fmt.Fprintln(stdout, "CDN purge completed.")
	return app.RolloutStatusComplete, nil
}
