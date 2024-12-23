package cloudmonitoring

import (
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sync"
	"time"
)

func TimeSeriesFetcherFromMapping(mapping MetricMapping, options workspace.MetricsGetterOptions, series *workspace.MetricSeries) *TimeSeriesFetcher {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name: fmt.Sprintf("projects/%s", mapping.ProjectId),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestampPtr(options.EndTime),
			StartTime: timestampPtr(options.StartTime),
		},
		Filter: mapping.ResourceFilter,
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod: durationpb.New(CalcPeriod(options.StartTime, options.EndTime)),
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}
	switch mapping.Aggregation {
	case "sum":
		req.Aggregation.PerSeriesAligner = monitoringpb.Aggregation_ALIGN_SUM
	case "average":
		req.Aggregation.PerSeriesAligner = monitoringpb.Aggregation_ALIGN_MEAN
	case "max":
		req.Aggregation.PerSeriesAligner = monitoringpb.Aggregation_ALIGN_MAX
	case "min":
		req.Aggregation.PerSeriesAligner = monitoringpb.Aggregation_ALIGN_MIN
	}

	return &TimeSeriesFetcher{
		Request: req,
		Series:  series,
	}
}

type TimeSeriesFetcher struct {
	Request *monitoringpb.ListTimeSeriesRequest
	Series  *workspace.MetricSeries
	Error   error
}

func (f *TimeSeriesFetcher) Fetch(ctx context.Context, wg *sync.WaitGroup, client *monitoring.MetricClient) {
	defer wg.Done()

	for resp, err := range client.ListTimeSeries(ctx, f.Request).All() {
		if err != nil {
			f.Error = fmt.Errorf("error fetching metrics: %w", err)
			return
		}
		for _, p := range resp.Points {
			endTime := time.Now()
			if p.Interval.EndTime != nil {
				endTime = p.Interval.EndTime.AsTime()
			}
			f.Series.AddPoint(endTime, mapPointValue(p.Value))
		}
	}
}

func timestampPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func mapPointValue(typedValue *monitoringpb.TypedValue) float64 {
	switch val := typedValue.Value.(type) {
	case *monitoringpb.TypedValue_DoubleValue:
		return val.DoubleValue
	case *monitoringpb.TypedValue_Int64Value:
		return float64(val.Int64Value)
	default:
		return 0
	}
}
