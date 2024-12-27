package cloudmonitoring

import (
	"github.com/nullstone-io/deployment-sdk/workspace"
)

type MappingGroups []MappingGroup

type MappingGroup struct {
	Name     string                      `json:"name"`
	Type     workspace.MetricDatasetType `json:"type"`
	Unit     string                      `json:"unit"`
	Mappings map[string]MetricMapping    `json:"mappings"`
}

// MetricMapping is retrieved from a workspace's outputs and can be used to construct a PromQL query
type MetricMapping struct {
	Query string `json:"query"`
}
