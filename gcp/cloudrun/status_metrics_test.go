package cloudrun

import (
	"testing"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
)

func TestSynthesizeInstances(t *testing.T) {
	t.Run("no instances, no traffic -> empty", func(t *testing.T) {
		got := synthesizeInstances(&Revision{Name: "r", TrafficPercent: 0}, 0, 0)
		assert.Empty(t, got)
	})
	t.Run("no instances but serving -> single cold cell", func(t *testing.T) {
		got := synthesizeInstances(&Revision{Name: "r", TrafficPercent: 100}, 0, 0)
		assert.Equal(t, []Instance{{Id: "r-0", State: InstanceStateCold}}, got)
	})
	t.Run("active and idle map to warm and idle cells", func(t *testing.T) {
		got := synthesizeInstances(&Revision{Name: "r"}, 2, 1)
		assert.Equal(t, []Instance{
			{Id: "r-0", State: InstanceStateWarm},
			{Id: "r-1", State: InstanceStateWarm},
			{Id: "r-2", State: InstanceStateIdle},
		}, got)
	})
	t.Run("caps total at maxInstanceCells, preferring active", func(t *testing.T) {
		got := synthesizeInstances(&Revision{Name: "r"}, maxInstanceCells+10, 5)
		assert.Len(t, got, maxInstanceCells)
		for _, c := range got {
			assert.Equal(t, InstanceStateWarm, c.State)
		}
	})
	t.Run("caps idle once active fills the budget", func(t *testing.T) {
		got := synthesizeInstances(&Revision{Name: "r"}, maxInstanceCells-2, 10)
		assert.Len(t, got, maxInstanceCells)
		assert.Equal(t, InstanceStateIdle, got[maxInstanceCells-1].State)
	})
}

func TestRefineServiceState(t *testing.T) {
	t.Run("never overrides Failing", func(t *testing.T) {
		svc := &Service{State: ServiceStateFailing, RequestHealth: &RequestHealth{RequestsPerSecond: 0}}
		refineServiceState(svc)
		assert.Equal(t, ServiceStateFailing, svc.State)
	})
	t.Run("degraded on sustained 5xx", func(t *testing.T) {
		svc := &Service{State: ServiceStateHealthy, RequestHealth: &RequestHealth{RequestsPerSecond: 10, ErrorRatePercent: 8}}
		refineServiceState(svc)
		assert.Equal(t, ServiceStateDegraded, svc.State)
	})
	t.Run("cold when no traffic and no instances", func(t *testing.T) {
		svc := &Service{State: ServiceStateHealthy, RequestHealth: &RequestHealth{RequestsPerSecond: 0}}
		refineServiceState(svc)
		assert.Equal(t, ServiceStateCold, svc.State)
	})
	t.Run("healthy when serving below error threshold", func(t *testing.T) {
		svc := &Service{
			State:         ServiceStateHealthy,
			RequestHealth: &RequestHealth{RequestsPerSecond: 5, ErrorRatePercent: 1},
			Revisions:     []Revision{{InstanceCount: 2}},
		}
		refineServiceState(svc)
		assert.Equal(t, ServiceStateHealthy, svc.State)
	})
	t.Run("cold without request health when no instances", func(t *testing.T) {
		svc := &Service{State: ServiceStateHealthy, Revisions: []Revision{{InstanceCount: 0}}}
		refineServiceState(svc)
		assert.Equal(t, ServiceStateCold, svc.State)
	})
}

func TestLatestValue(t *testing.T) {
	t.Run("no points", func(t *testing.T) {
		_, ok := latestValue(&monitoringpb.TimeSeries{})
		assert.False(t, ok)
	})
	t.Run("reads newest double point", func(t *testing.T) {
		ts := &monitoringpb.TimeSeries{Points: []*monitoringpb.Point{
			{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 42.5}}},
			{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 1}}},
		}}
		v, ok := latestValue(ts)
		assert.True(t, ok)
		assert.Equal(t, 42.5, v)
	})
	t.Run("reads int64 point", func(t *testing.T) {
		ts := &monitoringpb.TimeSeries{Points: []*monitoringpb.Point{
			{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 7}}},
		}}
		v, ok := latestValue(ts)
		assert.True(t, ok)
		assert.Equal(t, 7.0, v)
	})
}
