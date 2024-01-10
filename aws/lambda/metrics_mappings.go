package lambda

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
)

var (
	MetricMappings = cloudwatch.MetricMappingGroup{
		"invocations": {
			"invocations_total": {
				Stat:      "Sum",
				Namespace: "AWS/Lambda",
				Name:      "Invocations",
			},
			"invocations_errors": {
				Stat:      "Sum",
				Namespace: "AWS/Lambda",
				Name:      "Errors",
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
