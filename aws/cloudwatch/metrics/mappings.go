package metrics

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"regexp"
	"slices"
	"sort"
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
			grpQueries := make([]types.MetricDataQuery, 0)
			// metricIdMappings provides a mapping of the original metric id to the generated metric id (e.g. "cpu_reserved" -> "group_0_cpu_reserved")
			// For queries that are expressions, we will replace the metric id with the generated metric id
			metricIdMappings := map[string]string{}
			for id, mapping := range grp.Mappings {
				query := mapping.ToMetricDateQuery(g.genMetricId(i, id), periodSec)
				metricIdMappings[id] = *query.Id
				grpQueries = append(grpQueries, query)
			}
			grpQueries = updateQueryExpressions(grpQueries, metricIdMappings)
			queries = append(queries, grpQueries...)
		}
	}
	sort.SliceStable(queries, func(i, j int) bool {
		hasExpr1 := queries[i].Expression != nil
		hasExpr2 := queries[j].Expression != nil
		if hasExpr1 == hasExpr2 {
			return *queries[i].Id < *queries[j].Id
		}
		return !hasExpr1
	})
	return queries
}

func (g MappingGroups) FindByMetricId(metricId string) (*MappingGroup, string, MetricMapping) {
	for i, grp := range g {
		for id, mapping := range grp.Mappings {
			if g.genMetricId(i, id) == metricId {
				return &grp, id, mapping
			}
		}
	}
	return nil, "", MetricMapping{}
}

func (g MappingGroups) genMetricId(i int, id string) string {
	return fmt.Sprintf("group_%d_%s", i, id)
}

// updateQueryExpressions alters any query expressions so that they use the correct metric id
// We have to do this because the "id" in our metric queries has a generated prefix
// We have to prefix the metric id to make each metric query unique to the API call
func updateQueryExpressions(grpQueries []types.MetricDataQuery, metricIdMappings map[string]string) []types.MetricDataQuery {
	for j, query := range grpQueries {
		if query.Expression != nil {
			modified := replaceExpressionWithGeneratedIds(*query.Expression, metricIdMappings)
			grpQueries[j].Expression = &modified
		}
	}
	return grpQueries
}

func replaceExpressionWithGeneratedIds(expr string, metricIdMappings map[string]string) string {
	result := expr
	for originalId, generatedId := range metricIdMappings {
		pattern := fmt.Sprintf(`(^|[^a-zA-Z0-9_])(%s)([^a-zA-Z0-9_]|$)`, regexp.QuoteMeta(originalId))
		if re, err := regexp.Compile(pattern); err == nil {
			// We are going to skip any bad regexes, the user must have a bad id anyways
			result = re.ReplaceAllString(result, fmt.Sprintf("${1}%s${3}", generatedId))
		}
	}
	return result
}
