package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// ClassifyContainer inspects a single container's status against the catalog.
// Returns nil when the container is healthy (no waiting failure, no terminated
// failure). The dispatch order is:
//
//  1. Image-related waiting reasons (cheapest; no message regexes)
//  2. Runtime / lifecycle (CrashLoopBackOff, OOMKilled, Create/RunContainerError, Terminated)
//
// The pod argument is used only to populate ObjectRef.Namespace/Name and to
// allow future cross-checks; pass corev1.Pod{} if you only have a status.
func ClassifyContainer(pod corev1.Pod, status corev1.ContainerStatus) *Failure {
	obj := ObjectRef{
		Kind:      "Pod",
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Container: status.Name,
	}
	if f := classifyImage(obj, status); f != nil {
		return f
	}
	if f := classifyRuntime(obj, status); f != nil {
		return f
	}
	return nil
}

// ClassifyPod returns failures derived from pod-level state — scheduling,
// eviction, disruption. It does NOT recurse into containers; call
// ClassifyContainer per container alongside this.
func ClassifyPod(pod corev1.Pod) []Failure {
	obj := ObjectRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
	var out []Failure
	if f := classifyPending(obj, pod); f != nil {
		out = append(out, *f)
	}
	if f := classifyNode(obj, pod); f != nil {
		out = append(out, *f)
	}
	return out
}

// ClassifyEvent maps a single corev1.Event to a Failure. Returns nil for events
// whose reason isn't in the catalog (callers can still log the raw event).
//
// The dispatch is by reason because the event API namespaces failures that way;
// message-based sub-classification happens inside the per-category function.
func ClassifyEvent(ev corev1.Event) *Failure {
	obj := ObjectRef{
		Kind:      ev.InvolvedObject.Kind,
		Namespace: ev.InvolvedObject.Namespace,
		Name:      ev.InvolvedObject.Name,
	}
	switch ev.Reason {
	case "FailedMount", "FailedAttachVolume", "ProvisioningFailed", "FailedBinding", "VolumeResizeFailed":
		return setObserved(classifyVolumeEvent(obj, ev), ev)
	case "FailedCreatePodSandBox", "SyncLoadBalancerFailed", "CreatingLoadBalancerFailed", "Unhealthy":
		return setObserved(classifyNetworkEvent(obj, ev), ev)
	case "FailedCreate":
		return setObserved(classifyAdmissionEvent(obj, ev), ev)
	case "FailedScheduling":
		// Equivalent to PodScheduled=False:Unschedulable on the pod, but emitted
		// as an event by the scheduler. Reuse the same message classifier.
		return setObserved(classifySchedulingMessage(obj, ev.Message), ev)
	case "Evicted":
		// Pod-level eviction also fires an event; reuse the node classifier
		// against a synthesized pod with status.reason=Evicted.
		fake := corev1.Pod{Status: corev1.PodStatus{Reason: "Evicted", Message: ev.Message}}
		return setObserved(classifyNode(obj, fake), ev)
	}
	return nil
}

// setObserved populates Failure.ObservedAt from the event's most recent timestamp.
// Helper to avoid threading event context through every classifier.
func setObserved(f *Failure, ev corev1.Event) *Failure {
	if f == nil {
		return nil
	}
	if !ev.LastTimestamp.Time.IsZero() {
		f.ObservedAt = ev.LastTimestamp.Time
	} else if !ev.EventTime.Time.IsZero() {
		f.ObservedAt = ev.EventTime.Time
	}
	return f
}

// containsAny reports whether s contains any of the given substrings.
// Used by message-based classifiers to keep regex-free, allocation-light dispatch.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
