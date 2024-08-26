package metrics

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"slices"
	"strconv"
	"strings"
)

type MappingGroups []MappingGroup

type MappingGroup struct {
	Name     string                      `json:"name"`
	Type     workspace.MetricDatasetType `json:"type"`
	Unit     string                      `json:"unit"`
	Mappings map[string]MetricMapping    `json:"mappings"`
}

type MetricMapping struct {
	AccountId       string                  `json:"account_id"`
	Stat            string                  `json:"stat"`
	Namespace       string                  `json:"namespace"`
	MetricName      string                  `json:"metric_name"`
	Dimensions      MetricMappingDimensions `json:"dimensions"`
	Expression      string                  `json:"expression"`
	HideFromResults bool                    `json:"hide_from_results"`
}

func (m MetricMapping) ToMetricDateQuery(id string, periodSec int32) types.MetricDataQuery {
	query := types.MetricDataQuery{
		Id:        aws.String(id),
		AccountId: aws.String(m.AccountId),
	}
	if m.Expression != "" {
		query.Expression = aws.String(m.Expression)
	} else {
		query.MetricStat = &types.MetricStat{
			Period: aws.Int32(periodSec),
			Stat:   aws.String(m.Stat),
			Metric: &types.Metric{
				Namespace:  aws.String(m.Namespace),
				MetricName: aws.String(m.MetricName),
				Dimensions: m.Dimensions.ToAws(),
			},
		}
	}
	return query
}

type MetricMappingDimensions map[string]string

func (d MetricMappingDimensions) ToAws() []types.Dimension {
	dims := make([]types.Dimension, 0)
	for k, v := range d {
		dims = append(dims, types.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}
	return dims
}

func (g MappingGroups) BuildMetricQueries(metrics []string, periodSec int32) []types.MetricDataQuery {
	queries := make([]types.MetricDataQuery, 0)
	for i, grp := range g {
		if len(metrics) < 1 || slices.Contains(metrics, grp.Name) {
			// This Metric Group was specified in the list of requested metrics
			// Let's build a query and add it
			for id, mapping := range grp.Mappings {
				uniqueId := fmt.Sprintf("group_%d_%s", i, id)
				queries = append(queries, mapping.ToMetricDateQuery(uniqueId, periodSec))
			}
		}
	}
	return queries
}

func (g MappingGroups) FindGroupByMetricId(metricId string) *MappingGroup {
	i, id := g.parseMetricId(metricId)
	if i >= 0 && i < len(g) {
		grp := g[i]
		if _, ok := grp.Mappings[id]; ok {
			return &grp
		}
	}
	return nil
}

// parseMetricId takes the unique metric id in the format `group_<i>_<id>` and extracts each component
// This returns (-1, "") if the format is invalid
func (g MappingGroups) parseMetricId(metricId string) (int, string) {
	rest, found := strings.CutPrefix(metricId, "group_")
	if !found {
		return -1, ""
	}
	istr, id, found := strings.Cut(rest, "_")
	if !found {
		return -1, ""
	}
	i, err := strconv.Atoi(istr)
	if err != nil {
		return -1, ""
	}
	if i < 0 || i >= len(g) {
		return -1, ""
	}
	return i, id
}
