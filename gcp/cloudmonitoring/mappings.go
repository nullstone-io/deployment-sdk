package cloudmonitoring

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch/metrics"
	"github.com/nullstone-io/deployment-sdk/workspace"
)

type MappingGroups []metrics.MappingGroup

type MappingGroup struct {
	Name     string                      `json:"name"`
	Type     workspace.MetricDatasetType `json:"type"`
	Unit     string                      `json:"unit"`
	Mappings map[string]MetricMapping    `json:"mappings"`
}

type MetricMapping struct {
	ProjectId      string `json:"project_id"`
	MetricName     string `json:"metric_name"`
	Aggregation    string `json:"aggregation"`
	ResourceFilter string `json:"resource_filter"`
}
