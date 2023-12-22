package cloudwatch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"time"
)

type IngestMetricPageFunc func(output *cloudwatch.GetMetricDataOutput) error

type GetMetricsOptions struct {
	StartTime *time.Time
	EndTime   *time.Time
	Queries   []types.MetricDataQuery
}

func GetMetrics(ctx context.Context, awsConfig aws.Config, options GetMetricsOptions, ingestFn IngestMetricPageFunc) error {
	cwClient := cloudwatch.NewFromConfig(awsConfig)

	var nextToken *string
	for {
		input := &cloudwatch.GetMetricDataInput{
			StartTime:         options.StartTime,
			EndTime:           options.EndTime,
			LabelOptions:      &types.LabelOptions{Timezone: aws.String("+0000")}, // UTC
			ScanBy:            types.ScanByTimestampAscending,
			NextToken:         nextToken,
			MetricDataQueries: options.Queries,
		}
		out, err := cwClient.GetMetricData(ctx, input)
		if err != nil {
			return fmt.Errorf("error retrieving metrics data: %w", err)
		}
		if err := ingestFn(out); err != nil {
			return err
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}

	return nil
}
