package azuremonitor

import (
	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	WorkspaceId string          `ns:"log_analytics_workspace_id"`
	LogFilter   string          `ns:"log_filter,optional"`
	LogReader   azure.Principal `ns:"log_reader"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.LogReader.InitializeCreds(source, ws, types.AutomationPurposeViewLogs, "log_reader")
}
