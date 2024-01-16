package gke

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"k8s.io/client-go/rest"
)

func NewLogStreamer(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.LogStreamer, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return k8s.LogStreamer{
		OsWriters:    osWriters,
		Details:      appDetails,
		AppNamespace: outs.ServiceNamespace,
		AppName:      outs.ServiceName,
		NewConfigFn: func(ctx context.Context) (*rest.Config, error) {
			return CreateKubeConfig(ctx, outs.ClusterNamespace, outs.Deployer)
		},
	}, nil
}
