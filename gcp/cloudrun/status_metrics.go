package cloudrun

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// metricsWindow is the look-back for all Cloud Monitoring queries. Cloud Run
	// samples at ~60s, so a short window keeps numbers current while still
	// capturing at least one aligned point.
	metricsWindow = 5 * time.Minute
	// metricsAlignment is the alignment period applied to every aligned series.
	metricsAlignment = 60 * time.Second
	// maxInstanceCells caps the synthesized instance cells per revision so a
	// service scaled to thousands of instances doesn't bloat the payload.
	maxInstanceCells = 50
	// degradedErrorRatePercent is the 5xx rate above which a serving service is
	// reported Degraded rather than Healthy.
	degradedErrorRatePercent = 5.0

	metricInstanceCount  = "run.googleapis.com/container/instance_count"
	metricRequestCount   = "run.googleapis.com/request_count"
	metricRequestLatency = "run.googleapis.com/request_latencies"
)

// enrichServiceMetrics augments a Service (already populated from the Run v2
// API) with live data from Cloud Monitoring: per-revision instance counts and
// synthesized instance cells, plus request-rate / latency / error-rate fields
// at both the revision and service level.
//
// All enrichment is best-effort. The Run v2 API exposes no live instance count
// or request health, but Cloud Monitoring requires roles/monitoring.viewer,
// which the deployer service account may lack. On any error we log to stderr
// and leave the Phase 1 data intact rather than failing the whole status call.
func (s Statuser) enrichServiceMetrics(ctx context.Context, svc *Service) {
	if svc == nil || len(svc.Revisions) == 0 {
		return
	}
	stderr := s.OsWriters.Stderr()
	loc := s.Infra.Location()
	if loc.ProjectId == "" {
		fmt.Fprintln(stderr, "cloud run metrics: skipping enrichment (no project id)")
		return
	}

	client, err := NewMetricClient(ctx, s.Infra.Deployer)
	if err != nil {
		fmt.Fprintf(stderr, "cloud run metrics: skipping enrichment (client init failed): %s\n", err)
		return
	}
	defer client.Close()

	m := metricsEnricher{
		client:      client,
		projectName: "projects/" + loc.ProjectId,
		serviceName: svc.ServiceName,
		location:    loc.Region,
		stderr:      stderr,
	}
	m.applyInstanceCounts(ctx, svc)
	m.applyRequestMetrics(ctx, svc)
	refineServiceState(svc)
}

// refineServiceState downgrades a Healthy service to Cold or Degraded based on
// the enriched metrics. It never overrides a Failing state (a revision failed
// to start) — that signal is stronger than any traffic-derived heuristic.
func refineServiceState(svc *Service) {
	if svc.State == ServiceStateFailing {
		return
	}

	var instances int32
	for _, rev := range svc.Revisions {
		instances += rev.InstanceCount
	}
	if rh := svc.RequestHealth; rh != nil {
		// Sustained 5xx rate on a service taking traffic: serving, but degraded.
		if rh.RequestsPerSecond > 0 && rh.ErrorRatePercent >= degradedErrorRatePercent {
			svc.State = ServiceStateDegraded
			return
		}
		// No traffic and no warm instances: scaled to zero.
		if rh.RequestsPerSecond == 0 && instances == 0 {
			svc.State = ServiceStateCold
			return
		}
	} else if instances == 0 {
		svc.State = ServiceStateCold
	}
}

type metricsEnricher struct {
	client      *monitoring.MetricClient
	projectName string
	serviceName string
	location    string
	stderr      io.Writer
}

// query runs a ListTimeSeries call scoped to this service+location and returns
// the resulting (reduced) series. Returns nil on error (logged) so callers
// degrade gracefully.
func (m metricsEnricher) query(ctx context.Context, metricType string, aligner monitoringpb.Aggregation_Aligner, reducer monitoringpb.Aggregation_Reducer, groupBy []string) []*monitoringpb.TimeSeries {
	now := time.Now()
	filter := fmt.Sprintf(`metric.type=%q AND resource.labels.service_name=%q`, metricType, m.serviceName)
	if m.location != "" {
		filter += fmt.Sprintf(` AND resource.labels.location=%q`, m.location)
	}
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   m.projectName,
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(now.Add(-metricsWindow)),
			EndTime:   timestamppb.New(now),
		},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(metricsAlignment),
			PerSeriesAligner:   aligner,
			CrossSeriesReducer: reducer,
			GroupByFields:      groupBy,
		},
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}

	out := make([]*monitoringpb.TimeSeries, 0)
	it := m.client.ListTimeSeries(ctx, req)
	for {
		ts, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			fmt.Fprintf(m.stderr, "cloud run metrics: query for %s failed: %s\n", metricType, err)
			return nil
		}
		out = append(out, ts)
	}
	return out
}

// applyInstanceCounts populates each revision's live InstanceCount and
// synthesizes instance cells. The instance_count gauge carries a `state` label
// of "active" (serving) or "idle" (allocated but parked); we map active to Warm
// cells and idle to Idle cells.
func (m metricsEnricher) applyInstanceCounts(ctx context.Context, svc *Service) {
	series := m.query(ctx, metricInstanceCount,
		monitoringpb.Aggregation_ALIGN_MEAN,
		monitoringpb.Aggregation_REDUCE_SUM,
		[]string{"resource.label.revision_name", "metric.label.state"})
	if series == nil {
		return
	}

	type counts struct{ active, idle int32 }
	byRev := map[string]*counts{}
	for _, ts := range series {
		rev := ts.GetResource().GetLabels()["revision_name"]
		if rev == "" {
			continue
		}
		v, ok := latestValue(ts)
		if !ok {
			continue
		}
		c := byRev[rev]
		if c == nil {
			c = &counts{}
			byRev[rev] = c
		}
		n := int32(math.Round(v))
		if ts.GetMetric().GetLabels()["state"] == "idle" {
			c.idle += n
		} else {
			c.active += n
		}
	}

	for i := range svc.Revisions {
		rev := &svc.Revisions[i]
		c := byRev[rev.Name]
		if c == nil {
			continue
		}
		rev.InstanceCount = c.active + c.idle
		rev.Instances = synthesizeInstances(rev, c.active, c.idle)
	}
}

// synthesizeInstances builds instance cells for the revision card. Cloud Run
// reports only aggregate active/idle counts, not per-instance identity, so we
// fabricate stable cell ids. Counts are capped at maxInstanceCells, preferring
// to show active instances.
func synthesizeInstances(rev *Revision, active, idle int32) []Instance {
	if active+idle <= 0 {
		// Serving traffic but no instances reported yet: mid cold-start.
		if rev.TrafficPercent > 0 {
			return []Instance{{Id: fmt.Sprintf("%s-0", rev.Name), State: InstanceStateCold}}
		}
		return make([]Instance, 0)
	}
	if active > maxInstanceCells {
		active = maxInstanceCells
	}
	if active+idle > maxInstanceCells {
		idle = maxInstanceCells - active
	}

	cells := make([]Instance, 0, active+idle)
	idx := 0
	for i := int32(0); i < active; i++ {
		cells = append(cells, Instance{Id: fmt.Sprintf("%s-%d", rev.Name, idx), State: InstanceStateWarm})
		idx++
	}
	for i := int32(0); i < idle; i++ {
		cells = append(cells, Instance{Id: fmt.Sprintf("%s-%d", rev.Name, idx), State: InstanceStateIdle})
		idx++
	}
	return cells
}

// applyRequestMetrics populates per-revision request rate, error rate, and
// latency percentiles, then rolls them up into the service-level RequestHealth.
func (m metricsEnricher) applyRequestMetrics(ctx context.Context, svc *Service) {
	// Request rate per revision and response-code class. ALIGN_RATE yields a
	// per-second rate; summing the per-class rates gives total rps, and the 5xx
	// share gives the error rate.
	rateSeries := m.query(ctx, metricRequestCount,
		monitoringpb.Aggregation_ALIGN_RATE,
		monitoringpb.Aggregation_REDUCE_SUM,
		[]string{"resource.label.revision_name", "metric.label.response_code_class"})

	type reqAgg struct{ total, errors float64 }
	byRev := map[string]*reqAgg{}
	for _, ts := range rateSeries {
		rev := ts.GetResource().GetLabels()["revision_name"]
		if rev == "" {
			continue
		}
		v, ok := latestValue(ts)
		if !ok {
			continue
		}
		a := byRev[rev]
		if a == nil {
			a = &reqAgg{}
			byRev[rev] = a
		}
		a.total += v
		if ts.GetMetric().GetLabels()["response_code_class"] == "5xx" {
			a.errors += v
		}
	}

	// Latency percentiles. Aligning each distribution with ALIGN_DELTA and then
	// reducing with REDUCE_PERCENTILE_* merges the underlying distributions and
	// computes a true percentile (in milliseconds) per group.
	p50 := m.percentileByRevision(ctx, monitoringpb.Aggregation_REDUCE_PERCENTILE_50)
	p95 := m.percentileByRevision(ctx, monitoringpb.Aggregation_REDUCE_PERCENTILE_95)

	var totalRps, totalErrors float64
	for i := range svc.Revisions {
		rev := &svc.Revisions[i]
		if a := byRev[rev.Name]; a != nil {
			rev.RequestsPerSecond = ptr(a.total)
			errRate := 0.0
			if a.total > 0 {
				errRate = a.errors / a.total * 100
			}
			rev.ErrorRatePercent = ptr(errRate)
			totalRps += a.total
			totalErrors += a.errors
		}
		if v, ok := p50[rev.Name]; ok {
			rev.P50Ms = ptr(v)
		}
		if v, ok := p95[rev.Name]; ok {
			rev.P95Ms = ptr(v)
		}
	}

	// Service-level health. rps/error-rate roll up from the per-revision rates;
	// latency percentiles are computed over the merged distribution across all
	// revisions (percentiles are not additive, so they can't be summed).
	health := RequestHealth{RequestsPerSecond: totalRps}
	if totalRps > 0 {
		health.ErrorRatePercent = totalErrors / totalRps * 100
	}
	if v, ok := m.percentileOverall(ctx, monitoringpb.Aggregation_REDUCE_PERCENTILE_50); ok {
		health.P50Ms = v
	}
	if v, ok := m.percentileOverall(ctx, monitoringpb.Aggregation_REDUCE_PERCENTILE_95); ok {
		health.P95Ms = v
	}
	svc.RequestHealth = &health
}

func (m metricsEnricher) percentileByRevision(ctx context.Context, reducer monitoringpb.Aggregation_Reducer) map[string]float64 {
	out := map[string]float64{}
	for _, ts := range m.query(ctx, metricRequestLatency, monitoringpb.Aggregation_ALIGN_DELTA, reducer, []string{"resource.label.revision_name"}) {
		rev := ts.GetResource().GetLabels()["revision_name"]
		if rev == "" {
			continue
		}
		if v, ok := latestValue(ts); ok {
			out[rev] = v
		}
	}
	return out
}

func (m metricsEnricher) percentileOverall(ctx context.Context, reducer monitoringpb.Aggregation_Reducer) (float64, bool) {
	for _, ts := range m.query(ctx, metricRequestLatency, monitoringpb.Aggregation_ALIGN_DELTA, reducer, nil) {
		if v, ok := latestValue(ts); ok {
			return v, true
		}
	}
	return 0, false
}

// latestValue returns the most recent point's value as a float64. ListTimeSeries
// returns points newest-first, so index 0 is the latest aligned point.
func latestValue(ts *monitoringpb.TimeSeries) (float64, bool) {
	pts := ts.GetPoints()
	if len(pts) == 0 {
		return 0, false
	}
	v := pts[0].GetValue()
	if v == nil {
		return 0, false
	}
	switch v.GetValue().(type) {
	case *monitoringpb.TypedValue_Int64Value:
		return float64(v.GetInt64Value()), true
	default:
		return v.GetDoubleValue(), true
	}
}

func ptr[T any](v T) *T {
	return &v
}
