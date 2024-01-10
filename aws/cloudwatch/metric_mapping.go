package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// .[metric-group] => [metrid-id] => mapping
type MetricMappingGroup map[string]map[string]MetricMapping

type MetricMapping struct {
	Stat      string
	Namespace string
	Name      string
}

type MappingContext struct {
	AccountId  string
	PeriodSec  int32
	Dimensions []types.Dimension
}

func (g MetricMappingGroup) BuildMetricQueries(metrics []string, mappingCtx MappingContext) []types.MetricDataQuery {
	queries := make([]types.MetricDataQuery, 0)
	for _, metric := range metrics {
		if grp, ok := g[metric]; ok {
			for id, mapping := range grp {
				queries = append(queries, types.MetricDataQuery{
					Id:        aws.String(id),
					AccountId: aws.String(mappingCtx.AccountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(mappingCtx.PeriodSec),
						Stat:   aws.String(mapping.Stat),
						Metric: &types.Metric{
							Namespace:  aws.String(mapping.Namespace),
							MetricName: aws.String(mapping.Name),
							Dimensions: mappingCtx.Dimensions,
						},
					},
				})
			}
		}
	}
	return queries
}

func (g MetricMappingGroup) FindGroupByMetricId(id string) string {
	for grp, mappings := range g {
		if _, ok := mappings[id]; ok {
			return grp
		}
	}
	return ""
}
