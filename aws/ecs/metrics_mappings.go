package ecs

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/block"
)

var (
	MetricMappings = cloudwatch.MetricMappingGroups{
		{
			Name: "cpu",
			Type: block.MetricDatasetTypeUsage,
			Mappings: map[string]cloudwatch.MetricMapping{
				"cpu_reserved": {
					Stat:      "Average",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuReserved",
				},
				"cpu_average": {
					Stat:      "Average",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
				"cpu_min": {
					Stat:      "Minimum",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
				"cpu_max": {
					Stat:      "Maximum",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
			},
		},
		{
			Name: "memory",
			Type: block.MetricDatasetTypeUsage,
			Mappings: map[string]cloudwatch.MetricMapping{
				"cpu_reserved": {
					Stat:      "Average",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuReserved",
				},
				"cpu_average": {
					Stat:      "Average",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
				"cpu_min": {
					Stat:      "Minimum",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
				"cpu_max": {
					Stat:      "Maximum",
					Namespace: "ECS/ContainerInsights",
					Name:      "CpuUtilized",
				},
			},
		},
	}
)
