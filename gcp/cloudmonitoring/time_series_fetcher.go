package cloudmonitoring

import (
	"context"
	"sync"
	"time"

	"github.com/nullstone-io/deployment-sdk/prometheus"
	"github.com/nullstone-io/deployment-sdk/workspace"
)

// TimeSeriesFetcherFromMapping creates an interface for fetching the metrics in a background goroutine
// This fetcher uses the QueryClient to fetch metrics using MQL
func TimeSeriesFetcherFromMapping(mapping MetricMapping, options workspace.MetricsGetterOptions, series *workspace.MetricSeries) *TimeSeriesFetcher {
	end := time.Now()
	start := end.Add(-time.Hour)
	if options.StartTime != nil {
		start = *options.StartTime
	}
	if options.EndTime != nil {
		end = *options.EndTime
	}

	qo := prometheus.QueryOptions{
		Start: start,
		End:   end,
		IntervalCalculator: prometheus.NewIntervalCalculator(prometheus.IntervalOptions{
			Start:          start,
			End:            end,
			PanelWidth:     options.PanelWidth,
			ScrapeInterval: options.ScrapeInterval,
		}),
	}
	return &TimeSeriesFetcher{
		Query:   mapping.Query,
		Options: qo,
		Series:  series,
	}
}

type TimeSeriesFetcher struct {
	Query   string
	Options prometheus.QueryOptions
	Series  *workspace.MetricSeries
	Error   error
}

func (f *TimeSeriesFetcher) Fetch(ctx context.Context, wg *sync.WaitGroup, client *prometheus.QueryClient) {
	defer wg.Done()

	res, err := client.Query(ctx, f.Query, f.Options)
	if err != nil {
		f.Error = err
		return
	}
	for _, set := range res.Data.Result {
		for i, _ := range set.Values {
			tp, err := set.GetTimePoint(i)
			if err != nil {
				f.Error = err
				return
			}
			f.Series.AddPoint(tp.Timestamp, tp.Value)
		}
	}
}
