package metrics

import (
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region          string        `ns:"region"`
	MetricsReader   nsaws.User    `ns:"metrics_reader,optional"`
	LogReader       nsaws.User    `ns:"log_reader,optional"`
	MetricsMappings MappingGroups `ns:"metrics_mappings"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.MetricsReader.RemoteProvider = credsFactory("metrics_reader", "log_reader")
}
