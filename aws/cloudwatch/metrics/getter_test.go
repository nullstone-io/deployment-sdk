package metrics

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nullstone-io/deployment-sdk/workspace"
)

func TestGetter_GetMetrics_NoQueries(t *testing.T) {
	tests := []struct {
		name    string
		infra   Outputs
		options workspace.MetricsGetterOptions
	}{
		{
			name:    "empty metrics mappings",
			infra:   Outputs{MetricsMappings: MappingGroups{}},
			options: workspace.MetricsGetterOptions{},
		},
		{
			name: "filter matches no mappings",
			infra: Outputs{
				MetricsMappings: MappingGroups{
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
						},
					},
				},
			},
			options: workspace.MetricsGetterOptions{Metrics: []string{"memory"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getter := Getter{Infra: test.infra}
			got, err := getter.GetMetrics(context.Background(), test.options)
			if err != nil {
				t.Fatalf("GetMetrics() unexpected error: %v", err)
			}
			if diff := cmp.Diff(workspace.NewMetricsData(), got); diff != "" {
				t.Errorf("GetMetrics() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
