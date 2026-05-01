package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyNode handles §11 (Node / Infrastructure) signals at the Pod level —
// specifically Evicted (node pressure) and Preempted. Node-NotReady itself
// surfaces as a Node-level condition; we don't read Node objects here.
func classifyNode(obj ObjectRef, pod corev1.Pod) *Failure {
	if pod.Status.Reason == "Evicted" || strings.Contains(strings.ToLower(pod.Status.Message), "the node was low on resource") {
		lower := strings.ToLower(pod.Status.Message)
		name := "Evicted/NodePressure"
		summary := "Pod was evicted due to node resource pressure"
		switch {
		case strings.Contains(lower, "ephemeral-storage"):
			name = "Evicted/EphemeralStorage"
			summary = "Pod evicted because the node ran out of ephemeral storage"
		case strings.Contains(lower, "memory"):
			name = "Evicted/Memory"
			summary = "Pod evicted because the node ran low on memory"
		case strings.Contains(lower, "imagefs"), strings.Contains(lower, "nodefs"):
			name = "Evicted/DiskPressure"
			summary = "Pod evicted because the node disk was full"
		case strings.Contains(lower, "pids"):
			name = "Evicted/PIDPressure"
			summary = "Pod evicted because the node ran out of PIDs"
		}
		return &Failure{
			Name:        name,
			Category:    CategoryNode,
			Summary:     summary,
			Remediation: "Set ephemeral-storage limits; reduce log/scratch usage; add capacity or re-balance workloads off the affected node.",
			Object:      obj,
			Signals:     Signals{EventReason: pod.Status.Reason, EventMessage: pod.Status.Message},
			Provider:    ProviderGeneric,
			Docs:        []string{"https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/"},
		}
	}

	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.DisruptionTarget && c.Status == corev1.ConditionTrue {
			return &Failure{
				Name:        "DisruptionTarget/" + c.Reason,
				Category:    CategoryNode,
				Summary:     "Pod is targeted for disruption (" + c.Reason + ")",
				Remediation: "Surface as informational; the controller has decided to terminate this pod.",
				Object:      obj,
				Signals:     Signals{Condition: "DisruptionTarget=True:" + c.Reason, EventMessage: c.Message},
				Provider:    ProviderGeneric,
			}
		}
	}
	return nil
}
