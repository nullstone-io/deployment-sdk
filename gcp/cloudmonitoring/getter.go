package cloudmonitoring

import (
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"context"
	"errors"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"google.golang.org/api/option"
	"sync"
)

var (
	_ workspace.MetricsGetter = Getter{}

	MetricScopes = []string{
		"https://www.googleapis.com/auth/monitoring.read",
	}
)

func NewGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, blockDetails workspace.Details) (workspace.MetricsGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, blockDetails.Workspace, blockDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
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
	tokenSource, err := g.Infra.MetricsReader.TokenSource(ctx, MetricScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := monitoring.NewMetricClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics client: %w", err)
	}
	defer client.Close()

	result := workspace.NewMetricsData()
	wg := &sync.WaitGroup{}
	fetchers := make([]*TimeSeriesFetcher, 0)
	for i, grp := range g.Infra.MetricsMappings {
		ds := result.GetDataset(grp.Name, grp.Type, grp.Unit)
		for id, mapping := range grp.Mappings {
			wg.Add(1)
			curSeries := ds.GetSeries(id, fmt.Sprintf("group_%d_%s", i, id))
			fetcher := TimeSeriesFetcherFromMapping(mapping, options, curSeries)
			fetchers = append(fetchers, fetcher)
			go fetcher.Fetch(ctx, wg, client)
		}
	}
	wg.Wait()
	errs := make([]error, 0)
	for _, fetcher := range fetchers {
		if fetcher.Error != nil {
			errs = append(errs, fetcher.Error)
		}
	}
	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}
