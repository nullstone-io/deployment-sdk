package cloudmonitoring

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/prometheus"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"sync"
	"time"
)

// TimeSeriesFetcherFromMapping creates an interface for fetching the metrics in a background goroutine
// This fetcher utilizes the QueryClient to fetch metrics using MQL
func TimeSeriesFetcherFromMapping(mapping MetricMapping, options workspace.MetricsGetterOptions, series *workspace.MetricSeries) *TimeSeriesFetcher {
	interval := CalcPeriod(options.StartTime, options.EndTime)
	steps := int(interval / time.Second)
	qo := prometheus.QueryOptions{
		Start: options.StartTime,
		End:   options.EndTime,
		Step:  steps,
	}
	if qo.Start == nil {
		start := time.Now().Add(-time.Hour)
		qo.Start = &start
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
