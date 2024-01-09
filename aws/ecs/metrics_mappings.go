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
					Id:        aws.String("cpu_reserved"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Average"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("CpuReserved"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("cpu_average"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Average"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("CpuUtilized"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("cpu_min"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Minimum"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("CpuUtilized"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("cpu_max"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Maximum"),
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
					Id:        aws.String("memory_reserved"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Average"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("MemoryReserved"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("memory_average"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Average"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("MemoryUtilized"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("memory_min"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Minimum"),
						Metric: &types.Metric{
							Namespace:  aws.String("ECS/ContainerInsights"),
							MetricName: aws.String("MemoryUtilized"),
							Dimensions: ecsServiceDims,
						},
					},
				},
				{
					Id:        aws.String("memory_max"),
					AccountId: aws.String(accountId),
					MetricStat: &types.MetricStat{
						Period: aws.Int32(periodSec),
						Stat:   aws.String("Maximum"),
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
		"cpu_reserved":    "cpu",
		"cpu_average":     "cpu",
		"cpu_min":         "cpu",
		"cpu_max":         "cpu",
		"memory_reserved": "memory",
		"memory_average":  "memory",
		"memory_min":      "memory",
		"memory_max":      "memory",
	}
)
