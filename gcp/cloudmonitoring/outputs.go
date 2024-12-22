package cloudmonitoring

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
)

type Outputs struct {
	MetricsReader   gcp.ServiceAccount `ns:"metrics_reader"`
	MetricsMappings MappingGroups      `ns:"metrics_mappings"`
}
