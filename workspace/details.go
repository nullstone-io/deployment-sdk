package workspace

import "gopkg.in/nullstone-io/go-api-client.v0/types"

type Details struct {
	Block     *types.Block
	Env       *types.Environment
	Workspace *types.Workspace
	Module    *types.Module
}
