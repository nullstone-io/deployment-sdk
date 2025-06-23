package metrics

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	smithydocument "github.com/aws/smithy-go/document"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"testing"
)

func TestMappingGroups_BuildMetricQueries(t *testing.T) {
	input := MappingGroups{
		{
			Name: "cpu",
			Type: "usage-percent",
			Unit: "%",
			Mappings: map[string]MetricMapping{
				"cpu_average": {
					Stat:       "Average",
					Namespace:  "AWS/RDS",
					MetricName: "CPUUtilization",
				},
				"cpu_min": {
					Stat:       "Minimum",
					Namespace:  "AWS/RDS",
					MetricName: "CPUUtilization",
				},
				"cpu_max": {
					Stat:       "Maximum",
					Namespace:  "AWS/RDS",
					MetricName: "CPUUtilization",
				},
			},
		},
		{
			Name: "memory",
			Type: "usage",
			Unit: "MB",
			Mappings: map[string]MetricMapping{
				"memory_average_bytes": {
					Stat:       "Average",
					Namespace:  "AWS/RDS",
					MetricName: "FreeableMemory",
				},
				"memory_average": {
					Expression: "memory_average_bytes / 1048576",
				},
			},
		},
	}
	got := input.BuildMetricQueries([]string{"cpu", "memory"}, 60)
	want := []types.MetricDataQuery{
		{
			Id:        aws.String("group_0_cpu_average"),
			AccountId: aws.String(""),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Dimensions: make([]types.Dimension, 0),
					MetricName: aws.String("CPUUtilization"),
					Namespace:  aws.String("AWS/RDS"),
				},
				Period: aws.Int32(60),
				Stat:   aws.String("Average"),
			},
		},
		{
			Id:        aws.String("group_0_cpu_max"),
			AccountId: aws.String(""),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Dimensions: make([]types.Dimension, 0),
					MetricName: aws.String("CPUUtilization"),
					Namespace:  aws.String("AWS/RDS"),
				},
				Period: aws.Int32(60),
				Stat:   aws.String("Maximum"),
			},
		},
		{
			Id:        aws.String("group_0_cpu_min"),
			AccountId: aws.String(""),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Dimensions: make([]types.Dimension, 0),
					MetricName: aws.String("CPUUtilization"),
					Namespace:  aws.String("AWS/RDS"),
				},
				Period: aws.Int32(60),
				Stat:   aws.String("Minimum"),
			},
		},
		{
			Id:        aws.String("group_1_memory_average_bytes"),
			AccountId: aws.String(""),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Dimensions: make([]types.Dimension, 0),
					MetricName: aws.String("FreeableMemory"),
					Namespace:  aws.String("AWS/RDS"),
				},
				Period: aws.Int32(60),
				Stat:   aws.String("Average"),
			},
		},
		{
			Id:         aws.String("group_1_memory_average"),
			AccountId:  aws.String(""),
			Expression: aws.String("group_1_memory_average_bytes / 1048576"),
		},
	}
	opts := []cmp.Option{cmpopts.IgnoreUnexported(), cmpopts.IgnoreTypes(smithydocument.NoSerde{})}
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("BuildMetricQueries() mismatch (-want +got):\n%s", diff)
	}
}
