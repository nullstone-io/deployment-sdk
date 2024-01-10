package lambda

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

// MetricsGetter retrieves metrics for a Lambda serverless app with the following datasets (filtered by options)
// invocations
// - invocations (no unit)
// duration
// - duration_average (ms)
// - duration_min (ms)
// - duration_max (ms)
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
				Name:  aws.String("FunctionName"),
				Value: aws.String(g.Infra.FunctionName()),
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

	err := cloudwatch.GetMetrics(ctx, g.Infra.DeployerAwsConfig(), cwOptions, ingest)
	return result, err
}
