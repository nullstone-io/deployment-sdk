package cloudmonitoring

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	ProjectId       string             `ns:"project_id"`
	MetricsReader   gcp.ServiceAccount `ns:"metrics_reader"`
	MetricsMappings MappingGroups      `ns:"metrics_mappings"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.MetricsReader.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.Uid, "metrics_reader")
}
