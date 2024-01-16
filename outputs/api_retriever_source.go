package outputs

import (
	"github.com/google/uuid"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var _ RetrieverSource = ApiRetrieverSource{}

type ApiRetrieverSource struct {
	Config api.Config
}

func (s ApiRetrieverSource) GetWorkspace(stackId, blockId, envId int64) (*types.Workspace, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.Workspaces().Get(stackId, blockId, envId)
}

func (s ApiRetrieverSource) GetCurrentOutputs(stackId int64, workspaceUid uuid.UUID, showSensitive bool) (types.Outputs, error) {
	nsClient := api.Client{Config: s.Config}
	return nsClient.WorkspaceOutputs().GetCurrent(stackId, workspaceUid, showSensitive)
}
