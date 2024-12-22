package cloudmonitoring

import (
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
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

	//TODO implement me
	panic("implement me")
}

func (g Getter) fetchGroupData() workspace.MetricDataset {
	monitoringpb.ListTimeSeriesRequest{}
}

func (g Getter) fetchSeries(ctx context.Context, client *monitoring.MetricClient, id string, mapping MetricMapping, options workspace.MetricsGetterOptions) (*workspace.MetricSeries, error) {
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

	curSeries := workspace.NewMetricSeries(id)
	//curSeries := result.GetDataset(metricGroup.Name, metricGroup.Type, metricGroup.Unit).GetSeries(id, metricId)
	for resp, err := range client.ListTimeSeries(ctx, req).All() {
		if err != nil {
			return nil, fmt.Errorf("error retrieving metrics data: %w", err)
		}
		for _, p := range resp.Points {
			endTime := time.Now()
			if p.Interval.EndTime != nil {
				endTime = p.Interval.EndTime.AsTime()
			}
			curSeries.AddPoint(endTime, mapPointValue(p.Value))
		}
	}
	return curSeries, nil
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
