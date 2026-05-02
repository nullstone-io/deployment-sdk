package k8s

import (
	"strings"
	"testing"
	"time"

	"github.com/nullstone-io/deployment-sdk/k8s/failures"
	"github.com/stretchr/testify/assert"
)

func TestDeployEvent_String_Classified(t *testing.T) {
	ts := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	t.Run("classified failure renders canonical name next to reason", func(t *testing.T) {
		de := DeployEvent{
			Timestamp: ts,
			Type:      EventTypeWarning,
			Reason:    "Unhealthy",
			Object:    "pod/foo",
			Message:   "Liveness probe failed: HTTP 500",
			Failure: &failures.Failure{
				Name:     "LivenessProbeFailed",
				Category: failures.CategoryRuntime,
			},
		}
		out := de.String()
		assert.Contains(t, out, "(Unhealthy → LivenessProbeFailed)")
		assert.Contains(t, out, "Liveness probe failed: HTTP 500")
	})

	t.Run("matching reason is not duplicated", func(t *testing.T) {
		de := DeployEvent{
			Timestamp: ts,
			Type:      EventTypeWarning,
			Reason:    "FailedScheduling",
			Object:    "pod/foo",
			Message:   "0/3 nodes",
			Failure: &failures.Failure{
				Name: "FailedScheduling", // same as Reason
			},
		}
		out := de.String()
		assert.Contains(t, out, "(FailedScheduling)")
		assert.False(t, strings.Contains(out, "→"), "expected no arrow when classified name == raw reason; got %q", out)
	})

	t.Run("unclassified event renders without arrow", func(t *testing.T) {
		de := DeployEvent{
			Timestamp: ts,
			Type:      EventTypeNormal,
			Reason:    "Pulled",
			Object:    "pod/foo",
			Message:   "Container image pulled",
		}
		out := de.String()
		assert.Contains(t, out, "(Pulled)")
		assert.False(t, strings.Contains(out, "→"))
	})
}
