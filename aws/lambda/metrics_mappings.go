package lambda

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
)

var (
	MetricMappings = cloudwatch.MetricMappingGroup{
		"invocations": {
			"invocations": {
				Stat:      "Sum",
				Namespace: "AWS/Lambda",
				Name:      "Invocations",
			},
		},
		"duration": {
			"duration_average": {
				Stat:      "Average",
				Namespace: "AWS/Lambda",
				Name:      "Duration",
			},
			"duration_min": {
				Stat:      "Minimum",
				Namespace: "AWS/Lambda",
				Name:      "Duration",
			},
			"duration_max": {
				Stat:      "Maximum",
				Namespace: "AWS/Lambda",
				Name:      "Duration",
			},
		},
	}
)
