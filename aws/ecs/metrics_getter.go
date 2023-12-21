package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/app"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/logging"
)

var _ app.MetricsGetter = MetricsGetter{}
var _ cloudwatch.MetricsQueriesBuilder = MetricsGetter{}

type MetricsGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (g MetricsGetter) Get(ctx context.Context, options app.MetricsGetterOptions) (*app.MetricsData, error) {
	awsConfig := nsaws.NewConfig(g.Infra.LogReader, g.Infra.Region)
	return cloudwatch.GetMetrics(ctx, awsConfig, g, options)
}

func (g MetricsGetter) Build(metrics []string) []types.MetricDataQuery {
	accountId := g.Infra.AccountId()
	periodSec := int32(5 * 60) // 5 minutes
	dims := []types.Dimension{
		{
			Name:  aws.String("ClusterName"),
			Value: aws.String(g.Infra.ClusterName()),
		},
		{
			Name:  aws.String("ServiceName"),
			Value: aws.String(g.Infra.ServiceName),
		},
	}

	queries := make([]types.MetricDataQuery, 0)
	for _, metric := range metrics {
		cur := g.buildMetricQuery(metric, accountId, periodSec, dims)
		if cur != nil {
			queries = append(queries, *cur)
		}
	}
	return queries
}

func (g MetricsGetter) buildMetricQuery(metric string, accountId string, periodSec int32, ecsServiceDims []types.Dimension) *types.MetricDataQuery {
	switch metric {
	case "memory-utilized":
		return &types.MetricDataQuery{
			Id:        aws.String(metric),
			AccountId: aws.String(accountId),
			MetricStat: &types.MetricStat{
				Period: aws.Int32(periodSec),
				Stat:   aws.String("Sum"),
				Metric: &types.Metric{
					Namespace:  aws.String("ECS/ContainerInsights"),
					MetricName: aws.String("MemoryUtilized"),
					Dimensions: ecsServiceDims,
				},
			},
		}
	case "memory-reserved":
		return &types.MetricDataQuery{
			Id:        aws.String(metric),
			AccountId: aws.String(accountId),
			MetricStat: &types.MetricStat{
				Period: aws.Int32(periodSec),
				Stat:   aws.String("Sum"),
				Metric: &types.Metric{
					Namespace:  aws.String("ECS/ContainerInsights"),
					MetricName: aws.String("MemoryReserved"),
					Dimensions: ecsServiceDims,
				},
			},
		}
	case "cpu-utilized":
		return &types.MetricDataQuery{
			Id:        aws.String(metric),
			AccountId: aws.String(accountId),
			MetricStat: &types.MetricStat{
				Period: aws.Int32(periodSec),
				Stat:   aws.String("Sum"),
				Metric: &types.Metric{
					Namespace:  aws.String("ECS/ContainerInsights"),
					MetricName: aws.String("CpuUtilized"),
					Dimensions: ecsServiceDims,
				},
			},
		}
	case "cpu-reserved":
		return &types.MetricDataQuery{
			Id:        aws.String(metric),
			AccountId: aws.String(accountId),
			MetricStat: &types.MetricStat{
				Period: aws.Int32(periodSec),
				Stat:   aws.String("Sum"),
				Metric: &types.Metric{
					Namespace:  aws.String("ECS/ContainerInsights"),
					MetricName: aws.String("CpuReserved"),
					Dimensions: ecsServiceDims,
				},
			},
		}
	}
	return nil
}
