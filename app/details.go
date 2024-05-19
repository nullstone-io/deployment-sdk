package app

import "gopkg.in/nullstone-io/go-api-client.v0/types"

type Details struct {
	App             *types.Application     `json:"app"`
	Env             *types.Environment     `json:"env"`
	Workspace       *types.Workspace       `json:"workspace"`
	WorkspaceConfig *types.WorkspaceConfig `json:"workspaceConfig"`
	Module          *types.Module          `json:"module"`
}
