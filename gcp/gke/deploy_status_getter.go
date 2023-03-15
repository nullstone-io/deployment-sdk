package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

func NewDeployStatusGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

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
}

func (d *DeployStatusGetter) initialize(ctx context.Context, reference string) error {
	return nil
}

func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	stdout := d.OsWriters.Stdout()

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No service name in app module. Skipping check for healthy.\n")
		return app.RolloutStatusComplete, nil
	}

	if err := d.initialize(ctx, reference); err != nil {
		return app.RolloutStatusUnknown, err
	}

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	// TODO: Implement
	return app.RolloutStatusComplete, nil
}
