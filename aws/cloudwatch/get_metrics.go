package cloudwatch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/block"
	"time"
)

type IngestMetricPageFunc func(output *cloudwatch.GetMetricDataOutput) error

type GetMetricsOptions struct {
	StartTime *time.Time
	EndTime   *time.Time
	Queries   []types.MetricDataQuery
}

func GetMetrics(ctx context.Context, mappings MetricMappingGroups, awsConfig aws.Config, options GetMetricsOptions) (*block.MetricsData, error) {
	result := block.NewMetricsData()
	ingestFn := func(output *cloudwatch.GetMetricDataOutput) error {
		for _, dataResult := range output.MetricDataResults {
			metricId := *dataResult.Id
			metricGroup := mappings.FindGroupByMetricId(metricId)
			if metricGroup == nil {
				// This shouldn't happen, it means we don't have a mapping from metric id to its dataset
				// Should we warn?
				continue
			}

			curSeries := result.GetDataset(metricGroup.Name, metricGroup.Type).GetSeries(metricId)
			for i := 0; i < len(dataResult.Timestamps); i++ {
				curSeries.AddPoint(dataResult.Timestamps[i], dataResult.Values[i])
			}
		}
		return nil
	}

	cwClient := cloudwatch.NewFromConfig(awsConfig)

	input := &cloudwatch.GetMetricDataInput{
		StartTime:         options.StartTime,
		EndTime:           options.EndTime,
		LabelOptions:      &types.LabelOptions{Timezone: aws.String("+0000")}, // UTC
		ScanBy:            types.ScanByTimestampAscending,
		MetricDataQueries: options.Queries,
	}
	paginator := cloudwatch.NewGetMetricDataPaginator(cwClient, input)
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error retrieving metrics data: %w", err)
		}
		if err := ingestFn(out); err != nil {
			return nil, err
		}
	}

	return result, nil
}
