package ecs

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
)

var (
	MetricMappings = cloudwatch.MetricMappingGroup{
		"cpu": {
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
		"memory": {},
	}
)
