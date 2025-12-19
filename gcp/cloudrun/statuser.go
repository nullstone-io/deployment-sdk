package cloudrun

import (
	"context"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

var (
	_ app.StatusOverviewResult = StatusOverview{}
)

type StatusOverview struct {
}

func (s StatusOverview) GetDeploymentVersions() []string {
	return make([]string, 0)
}

type Status struct {
}

var (
	_ app.Statuser = &Statuser{}
)

func NewStatuser(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Statuser, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return Statuser{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Statuser struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (s Statuser) StatusOverview(ctx context.Context) (app.StatusOverviewResult, error) {
	// TODO: Implement when ServiceName != ""
	return StatusOverview{}, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	// TODO: Implement when ServiceName != ""
	return Status{}, nil
}
