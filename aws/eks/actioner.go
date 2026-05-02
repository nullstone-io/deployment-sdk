package eks

import (
	"context"

	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"k8s.io/client-go/rest"
)

func NewActioner(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, blockDetails workspace.Details) (workspace.Actioner, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, blockDetails.Workspace, blockDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}

	ws := blockDetails.Workspace
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	outs.Deployer.RemoteProvider = credsFactory(types.AutomationPurposePerformAction, "deployer")

	return k8s.Actioner{
		OsWriters: osWriters,
		Namespace: outs.ServiceNamespace,
		AppName:   blockDetails.Block.Name,
		NewConfigFn: func(ctx context.Context) (*rest.Config, error) {
			return CreateKubeConfig(ctx, outs.ClusterNamespace, outs.Deployer)
		},
	}, nil
}
