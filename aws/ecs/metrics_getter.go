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
	"math"
	"time"
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
// - cpu_reserved
// - cpu_utilized
// memory
// - memory_reserved (MB)
// - memory_utilized (MB)
// ECS Container Insights: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-ECS.html
// Default metric resolution is 1 minute
type MetricsGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (g MetricsGetter) GetMetrics(ctx context.Context, options app.MetricsGetterOptions) (*app.MetricsData, error) {
	cwOptions := cloudwatch.GetMetricsOptions{
		StartTime: options.StartTime,
		EndTime:   options.EndTime,
		Queries:   g.BuildMetricQueries(options.Metrics, options.StartTime, options.EndTime),
	}

	result := app.NewMetricsData()
	ingest := func(output *cloudwatch2.GetMetricDataOutput) error {
		for _, dataResult := range output.MetricDataResults {
			metricId := *dataResult.Id
			metricName, ok := MetricDatasetNameFromMetricId[metricId]
			if !ok {
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

func (g MetricsGetter) BuildMetricQueries(metrics []string, start, end *time.Time) []types.MetricDataQuery {
	accountId := g.Infra.AccountId()
	periodSec := calcPeriod(start, end)
	dims := []types.Dimension{
		{
			Name:  aws.String("ClusterName"),
			Value: aws.String(g.Infra.ClusterName()),
		},
		{
			Name:  aws.String("ServiceName"),
			Value: aws.String(g.Infra.ServiceName),
		},
	}

	queries := make([]types.MetricDataQuery, 0)
	for _, metric := range metrics {
		if fn, ok := MetricQueries[metric]; ok {
			queries = append(queries, fn(accountId, periodSec, dims)...)
		}
	}
	return queries
}

// calcPeriod determines how much time between datapoints
// This result is in number of seconds that can be used against the AWS API GetMetricData
// If period is small, we collect too much data and impair performance (retrieval and render)
// Since this offers no meaningful benefit to the user, we calculate period based on the time window (end - start)
// We are aiming for 60 datapoints total (e.g. 1m period : 1h window)
// If time window results in a decimal period, we round (resulting in more than 60 datapoints, at most 29)
func calcPeriod(start *time.Time, end *time.Time) int32 {
	s, e := time.Now().Add(-time.Hour), time.Now()
	if start != nil {
		s = *start
	}
	if end != nil {
		e = *end
	}
	window := e.Sub(s)
	return int32(math.Round(window.Hours()) * 60)
}
