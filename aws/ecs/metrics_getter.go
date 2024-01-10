package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudwatch2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/app"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

var _ app.MetricsGetter = MetricsGetter{}

func NewMetricsGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.MetricsGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return MetricsGetter{
		OsWriters: osWriters,
		Details:   appDetails,
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
	Details   app.Details
	Infra     Outputs
}

func (g MetricsGetter) GetMetrics(ctx context.Context, options app.MetricsGetterOptions) (*app.MetricsData, error) {
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

	result := app.NewMetricsData()
	ingest := func(output *cloudwatch2.GetMetricDataOutput) error {
		for _, dataResult := range output.MetricDataResults {
			metricId := *dataResult.Id
			metricName := MetricMappings.FindGroupByMetricId(metricId)
			if metricName == "" {
				// This shouldn't happen, it means we don't have a mapping from metric id to its dataset
				// Should we warn?
				continue
			}

			curSeries := result.GetDataset(metricName).GetSeries(metricId)
			for i := 0; i < len(dataResult.Timestamps); i++ {
				curSeries.AddPoint(dataResult.Timestamps[i], dataResult.Values[i])
			}
		}
		return nil
	}

	err := cloudwatch.GetMetrics(ctx, nsaws.NewConfig(g.Infra.LogReader, g.Infra.Region), cwOptions, ingest)
	// TODO: Normalize series to have the same number of datapoints and ordered the same
	return result, err
}
