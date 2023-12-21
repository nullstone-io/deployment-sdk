package cloudwatch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nullstone-io/deployment-sdk/app"
)

type MetricsQueriesBuilder interface {
	Build(metrics []string) []types.MetricDataQuery
}

func GetMetrics(ctx context.Context, awsConfig aws.Config, builder MetricsQueriesBuilder, options app.MetricsGetterOptions) (*app.MetricsData, error) {
	cwClient := cloudwatch.NewFromConfig(awsConfig)

	result := app.NewMetricsData()

	queries := builder.Build(options.Metrics)
	var nextToken *string
	for {
		input := &cloudwatch.GetMetricDataInput{
			StartTime:         options.StartTime,
			EndTime:           options.EndTime,
			LabelOptions:      &types.LabelOptions{Timezone: aws.String("+0000")}, // UTC
			ScanBy:            types.ScanByTimestampAscending,
			NextToken:         nextToken,
			MetricDataQueries: queries,
		}
		out, err := cwClient.GetMetricData(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("error retrieving metrics data: %w", err)
		}

		for _, dataResult := range out.MetricDataResults {
			curMetric := result.Metric(*dataResult.Id)
			for i := 0; i < len(dataResult.Timestamps); i++ {
				curMetric.AddPoint(dataResult.Timestamps[i], dataResult.Values[i])

			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}

	return result, nil
}
