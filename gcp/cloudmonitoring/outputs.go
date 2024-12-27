package cloudmonitoring

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
)

type Outputs struct {
	ProjectId       string             `ns:"project_id"`
	MetricsReader   gcp.ServiceAccount `ns:"metrics_reader"`
	MetricsMappings MappingGroups      `ns:"metrics_mappings"`
}
