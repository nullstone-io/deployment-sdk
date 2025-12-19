package cloudrun

import (
	"context"
	"fmt"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployWatcher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployWatcher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	if outs.ServiceName != "" {
		return &ServiceDeployWatcher{
			OsWriters: osWriters,
			Details:   appDetails,
			Infra:     outs,
		}, nil
	}
	return &app.PollingDeployWatcher{
		OsWriters: osWriters,
		StatusGetter: &JobDeployLogger{
			OsWriters: osWriters,
			Details:   appDetails,
			Infra:     outs,
		},
	}, nil
}

var _ app.DeployWatcher = &ServiceDeployWatcher{}

type ServiceDeployWatcher struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (s ServiceDeployWatcher) Watch(ctx context.Context, reference string, isFirstDeploy bool) error {
	stderr := s.OsWriters.Stderr()
	fmt.Fprintln(stderr, "Nullstone does not support waiting for a healthy Cloud Run service.")
	return nil
}
