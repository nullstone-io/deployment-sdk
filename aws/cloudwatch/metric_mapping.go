package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/block"
	"k8s.io/utils/strings/slices"
)

type MetricMappingGroups []MetricMappingGroup

type MetricMappingGroup struct {
	Name     string
	Type     block.MetricDatasetType
	Mappings map[string]MetricMapping
}

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

func (g MetricMappingGroups) BuildMetricQueries(metrics []string, mappingCtx MappingContext) []types.MetricDataQuery {
	queries := make([]types.MetricDataQuery, 0)
	for _, grp := range g {
		if slices.Contains(metrics, grp.Name) {
			// This Metric Group was specified in the list of requested metrics
			// Let's build a query and add it
			for id, mapping := range grp.Mappings {
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

func (g MetricMappingGroups) FindGroupByMetricId(id string) *MetricMappingGroup {
	for _, grp := range g {
		if _, ok := grp.Mappings[id]; ok {
			return &grp
		}
	}
	return nil
}
