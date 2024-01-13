package lambda

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/block"
)

var (
	MetricMappings = cloudwatch.MetricMappingGroups{
		{
			Name: "invocations",
			Type: block.MetricDatasetTypeInvocations,
			Mappings: map[string]cloudwatch.MetricMapping{
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
		},
		{
			Name: "duration",
			Type: block.MetricDatasetTypeDuration,
			Mappings: map[string]cloudwatch.MetricMapping{
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
		},
	}
)
