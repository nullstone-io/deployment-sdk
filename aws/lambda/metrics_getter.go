package lambda

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

// MetricsGetter retrieves metrics for a Lambda serverless app with the following datasets (filtered by options)
// invocations
// - invocations_total (count)
// - invocations_errors (errors)
// duration
// - duration_average (ms)
// - duration_min (ms)
// - duration_max (ms)
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

	// TODO: Normalize series to have the same number of datapoints and ordered the same
	return cloudwatch.GetMetrics(ctx, MetricMappings, g.Infra.DeployerAwsConfig(), cwOptions)
}
