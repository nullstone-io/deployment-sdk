package cloudmonitoring

import (
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"log"
	"sync"
	"time"
)

// TimeSeriesFetcherFromMapping creates an interface for fetching the metrics in a background goroutine
// This fetcher utilizes the QueryClient to fetch metrics using MQL
func TimeSeriesFetcherFromMapping(mapping MetricMapping, options workspace.MetricsGetterOptions, series *workspace.MetricSeries) *TimeSeriesFetcher {
	interval := CalcPeriod(options.StartTime, options.EndTime)
	end := time.Now()
	if options.EndTime != nil {
		end = *options.EndTime
	}
	start := end.Add(-time.Hour)
	if options.StartTime != nil {
		start = *options.StartTime
	}
	req := &monitoringpb.QueryTimeSeriesRequest{
		Name:  fmt.Sprintf("projects/%s", mapping.ProjectId),
		Query: mapping.ConstructMQL(interval, start, end),
	}
	log.Println(req.Query)
	return &TimeSeriesFetcher{
		Request: req,
		Series:  series,
	}
}

type TimeSeriesFetcher struct {
	Request *monitoringpb.QueryTimeSeriesRequest
	Series  *workspace.MetricSeries
	Error   error
}

func (f *TimeSeriesFetcher) Fetch(ctx context.Context, wg *sync.WaitGroup, client *monitoring.QueryClient) {
	defer wg.Done()

	for resp, err := range client.QueryTimeSeries(ctx, f.Request).All() {
		if err != nil {
			f.Error = fmt.Errorf("error fetching metrics: %w", err)
			return
		}
		for _, p := range resp.PointData {
			endTime := time.Now()
			if p.TimeInterval.EndTime != nil {
				endTime = p.TimeInterval.EndTime.AsTime()
			}
			for _, tv := range p.Values {
				f.Series.AddPoint(endTime, mapPointValue(tv))
			}
		}
	}
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
