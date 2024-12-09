package outputs

import (
	"context"
	"github.com/google/uuid"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var _ RetrieverSource = ApiRetrieverSource{}

type ApiRetrieverSource struct {
	Config api.Config
}

func (s ApiRetrieverSource) GetWorkspace(ctx context.Context, stackId, blockId, envId int64) (*types.Workspace, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.Workspaces().Get(ctx, stackId, blockId, envId)
}

func (s ApiRetrieverSource) GetCurrentConfig(ctx context.Context, stackId, blockId, envId int64) (*types.WorkspaceConfig, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.WorkspaceConfigs().GetCurrent(ctx, stackId, blockId, envId)
}

func (s ApiRetrieverSource) GetCurrentOutputs(ctx context.Context, stackId int64, workspaceUid uuid.UUID, showSensitive bool) (types.Outputs, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.WorkspaceOutputs().GetCurrent(ctx, stackId, workspaceUid, showSensitive)
}

func (s ApiRetrieverSource) GetTemporaryCredentials(ctx context.Context, stackId int64, workspaceUid uuid.UUID, input api.GenerateCredentialsInput) (*types.OutputCredentials, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.WorkspaceOutputCredentials().Create(ctx, stackId, workspaceUid, input)
}
