package cloudcdn

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/operations"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/sync"
)

func NewDeployStatusGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &DeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type DeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs

	getter sync.MemoizedLoad[operations.Getter]
}

func (d *DeployStatusGetter) Close() {
	if getter := d.getter.Value(); getter != nil {
		getter.Close()
	}
}

func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	getter, err := d.getter.Load(func() (operations.Getter, error) {
		tokenSource, err := d.Infra.Deployer.TokenSource(ctx, CdnScopes...)
		if err != nil {
			return nil, fmt.Errorf("error creating token source from service account: %w", err)
		}
		return operations.NewGetter(tokenSource, reference), nil
	})
	if err != nil {
		return app.RolloutStatusUnknown, err
	}

	op, err := getter.Get(ctx)
	if err != nil {
		return app.RolloutStatusUnknown, err
	} else if op == nil {
		return app.RolloutStatusUnknown, fmt.Errorf("could not find invalidation")
	}
	return d.mapRolloutStatus(op), nil
}

func (d *DeployStatusGetter) mapRolloutStatus(op *computepb.Operation) app.RolloutStatus {
	stdout, stderr := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if op == nil || op.Status == nil {
		fmt.Fprintln(stderr, "Missing invalidation status")
		return app.RolloutStatusUnknown
	}
	switch *op.Status {
	default:
		fmt.Fprintf(stderr, "Unknown invalidation status: %s\n", *op.Status)
		return app.RolloutStatusUnknown
	case computepb.Operation_PENDING:
		fmt.Fprintln(stdout, "Pending invalidation...")
		return app.RolloutStatusInProgress
	case computepb.Operation_RUNNING:
		fmt.Fprintln(stdout, "Invalidating...")
		return app.RolloutStatusInProgress
	case computepb.Operation_DONE:
		fmt.Fprintln(stdout, "Invalidation completed.")
		return app.RolloutStatusComplete
	}
}
