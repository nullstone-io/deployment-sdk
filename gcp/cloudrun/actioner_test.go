package cloudrun

import (
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/stretchr/testify/assert"
)

func TestBuildTrafficTargets(t *testing.T) {
	t.Run("100 percent -> single revision target", func(t *testing.T) {
		got := buildTrafficTargets("svc-00012-abc", 100, "svc-00012-abc")
		assert.Equal(t, []*runpb.TrafficTarget{
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION, Revision: "svc-00012-abc", Percent: 100},
		}, got)
	})
	t.Run("partial -> remainder to latest ready revision", func(t *testing.T) {
		got := buildTrafficTargets("svc-00012-abc", 30, "svc-00013-def")
		assert.Equal(t, []*runpb.TrafficTarget{
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION, Revision: "svc-00012-abc", Percent: 30},
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION, Revision: "svc-00013-def", Percent: 70},
		}, got)
	})
	t.Run("partial -> remainder to LATEST when latest ready is unknown", func(t *testing.T) {
		got := buildTrafficTargets("svc-00012-abc", 30, "")
		assert.Equal(t, []*runpb.TrafficTarget{
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION, Revision: "svc-00012-abc", Percent: 30},
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST, Percent: 70},
		}, got)
	})
	t.Run("partial -> remainder to LATEST when latest ready is the target", func(t *testing.T) {
		got := buildTrafficTargets("svc-00012-abc", 40, "svc-00012-abc")
		assert.Equal(t, []*runpb.TrafficTarget{
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION, Revision: "svc-00012-abc", Percent: 40},
			{Type: runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST, Percent: 60},
		}, got)
	})
}

func TestExecutionName(t *testing.T) {
	a := Actioner{Infra: Outputs{JobId: "projects/p/locations/us-east1/jobs/migrate"}}
	assert.Equal(t, "projects/p/locations/us-east1/jobs/migrate/executions/migrate-abc12", a.executionName("migrate-abc12"))
}
