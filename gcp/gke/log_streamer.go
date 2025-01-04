package gke

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"k8s.io/client-go/rest"
)

func NewLogStreamer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.LogStreamer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return k8s.LogStreamer{
		OsWriters:    osWriters,
		Details:      appDetails,
		AppNamespace: outs.ServiceNamespace,
		AppName:      appDetails.App.Name,
		NewConfigFn: func(ctx context.Context) (*rest.Config, error) {
			return CreateKubeConfig(ctx, outs.ClusterNamespace, outs.Deployer)
		},
	}, nil
}
