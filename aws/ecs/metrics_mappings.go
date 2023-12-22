package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type MetricQueryFactory func(accountId string, periodSec int32, ecsServiceDims []types.Dimension) []types.MetricDataQuery

var (
	MetricQueries = map[string]MetricQueryFactory{
		"cpu": func(accountId string, periodSec int32, ecsServiceDims []types.Dimension) []types.MetricDataQuery {
			return []types.MetricDataQuery{
				{
					Id:        aws.String("cpu-reserved"),
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
				},
				{
					Id:        aws.String("cpu-utilized"),
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
				},
			}
		},
		"memory": func(accountId string, periodSec int32, ecsServiceDims []types.Dimension) []types.MetricDataQuery {
			return []types.MetricDataQuery{
				{
					Id:        aws.String("memory-reserved"),
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
				},
				{
					Id:        aws.String("memory-utilized"),
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
				},
			}
		},
	}
	MetricDatasetNameFromMetricId = map[string]string{
		"cpu-reserved":    "cpu",
		"cpu-utilized":    "cpu",
		"memory-reserved": "memory",
		"memory-utilized": "memory",
	}
)
