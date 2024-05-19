package workspace

import "gopkg.in/nullstone-io/go-api-client.v0/types"

type Details struct {
	Block           *types.Block           `json:"block"`
	Env             *types.Environment     `json:"env"`
	Workspace       *types.Workspace       `json:"workspace"`
	WorkspaceConfig *types.WorkspaceConfig `json:"workspaceConfig"`
	Module          *types.Module          `json:"module"`
}
