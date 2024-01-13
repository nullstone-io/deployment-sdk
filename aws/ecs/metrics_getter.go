package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/block"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

var _ block.MetricsGetter = MetricsGetter{}

func NewMetricsGetter(osWriters logging.OsWriters, nsConfig api.Config, blockDetails block.Details) (block.MetricsGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, blockDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return MetricsGetter{
		OsWriters: osWriters,
		Details:   blockDetails,
		Infra:     outs,
	}, nil
}

// MetricsGetter retrieves metrics for an ECS container app with the following datasets (filtered by options)
// cpu
// - cpu_reserved (vCPU)
// - cpu_average (vCPU)
// - cpu_min (vCPU)
// - cpu_max (vCPU)
// memory
// - memory_reserved (MiB)
// - memory_average (MiB)
// - memory_min (MiB)
// - memory_max (MiB)
// ECS Container Insights: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-ECS.html
// Default metric resolution is 1 minute
type MetricsGetter struct {
	OsWriters logging.OsWriters
	Details   block.Details
	Infra     Outputs
}

func (g MetricsGetter) GetMetrics(ctx context.Context, options block.MetricsGetterOptions) (*block.MetricsData, error) {
	mappingCtx := cloudwatch.MappingContext{
		AccountId: g.Infra.AccountId(),
		PeriodSec: nsaws.CalcPeriod(options.StartTime, options.EndTime),
		Dimensions: []types.Dimension{
			{
				Name:  aws.String("ClusterName"),
				Value: aws.String(g.Infra.ClusterName()),
			},
			{
				Name:  aws.String("ServiceName"),
				Value: aws.String(g.Infra.ServiceName),
			},
		},
	}

	cwOptions := cloudwatch.GetMetricsOptions{
		StartTime: options.StartTime,
		EndTime:   options.EndTime,
		Queries:   MetricMappings.BuildMetricQueries(options.Metrics, mappingCtx),
	}

	// TODO: Normalize series to have the same number of datapoints and ordered the same
	return cloudwatch.GetMetrics(ctx, MetricMappings, nsaws.NewConfig(g.Infra.LogReader, g.Infra.Region), cwOptions)
}
