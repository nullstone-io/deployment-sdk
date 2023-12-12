package gke

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"k8s.io/client-go/rest"
)

func NewLogStreamer(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.LogStreamer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
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
