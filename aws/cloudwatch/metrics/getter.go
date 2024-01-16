package metrics

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

var _ workspace.MetricsGetter = Getter{}

func NewGetter(osWriters logging.OsWriters, nsConfig api.Config, blockDetails workspace.Details) (workspace.MetricsGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, blockDetails.Workspace)
	if err != nil {
		return nil, workspace.MetricsNotSupportedError{InnerErr: err}
	}

	return Getter{
		OsWriters: osWriters,
		Details:   blockDetails,
		Infra:     outs,
	}, nil
}

type Getter struct {
	OsWriters logging.OsWriters
	Details   workspace.Details
	Infra     Outputs
}

func (g Getter) GetMetrics(ctx context.Context, options workspace.MetricsGetterOptions) (*workspace.MetricsData, error) {
	periodSec := nsaws.CalcPeriod(options.StartTime, options.EndTime)
	queries := g.Infra.MetricsMappings.BuildMetricQueries(options.Metrics, periodSec)
	input := &cloudwatch.GetMetricDataInput{
		StartTime:         options.StartTime,
		EndTime:           options.EndTime,
		LabelOptions:      &types.LabelOptions{Timezone: aws.String("+0000")}, // UTC
		ScanBy:            types.ScanByTimestampAscending,
		MetricDataQueries: queries,
	}

	cwClient := cloudwatch.NewFromConfig(g.Infra.AwsConfig())
	paginator := cloudwatch.NewGetMetricDataPaginator(cwClient, input)
	result := workspace.NewMetricsData()
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error retrieving metrics data: %w", err)
		}
		g.ingest(out, result)
	}
	// TODO: Normalize series to have the same number of datapoints and ordered the same
	return result, nil
}

func (g Getter) ingest(output *cloudwatch.GetMetricDataOutput, result *workspace.MetricsData) {
	for _, dataResult := range output.MetricDataResults {
		metricId := *dataResult.Id
		metricGroup := g.Infra.MetricsMappings.FindGroupByMetricId(metricId)
		if metricGroup == nil {
			// This shouldn't happen, it means we don't have a mapping from metric id to its dataset
			// Should we warn?
			continue
		}
		if metricGroup.Mappings[metricId].HideFromResults {
			continue
		}
		curSeries := result.GetDataset(metricGroup.Name, metricGroup.Type, metricGroup.Unit).GetSeries(metricId)
		for i := 0; i < len(dataResult.Timestamps); i++ {
			curSeries.AddPoint(dataResult.Timestamps[i], dataResult.Values[i])
		}
	}
}
