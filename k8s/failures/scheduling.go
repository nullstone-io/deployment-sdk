package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyPending handles §3 (Scheduling) by interpreting Pod.status.conditions
// when the pod is stuck in Pending. Returns nil for pods that aren't pending or
// whose PodScheduled condition isn't reporting a recognized failure.
func classifyPending(obj ObjectRef, pod corev1.Pod) *Failure {
	if pod.Status.Phase != corev1.PodPending {
		return nil
	}
	for _, c := range pod.Status.Conditions {
		if c.Type != corev1.PodScheduled || c.Status == corev1.ConditionTrue {
			continue
		}
		switch c.Reason {
		case "Unschedulable":
			return classifySchedulingMessage(obj, c.Message)
		case "SchedulingGated":
			return &Failure{
				Name:        "SchedulingGated",
				Category:    CategoryScheduling,
				Summary:     "Pod has unresolved schedulingGates set by an external controller",
				Remediation: "The controller that owns the gate (Kueue, Karpenter, etc.) hasn't cleared it yet — investigate that controller.",
				Object:      obj,
				Signals:     Signals{Condition: "PodScheduled=False:SchedulingGated", EventMessage: c.Message},
				Provider:    ProviderGeneric,
			}
		}
	}
	return nil
}

// classifySchedulingMessage interprets a kube-scheduler "Unschedulable" message
// (e.g. "0/3 nodes are available: 3 Insufficient cpu") into a specific bucket.
func classifySchedulingMessage(obj ObjectRef, msg string) *Failure {
	lower := strings.ToLower(msg)
	signals := Signals{Condition: "PodScheduled=False:Unschedulable", EventReason: "FailedScheduling", EventMessage: msg}

	switch {
	case containsAny(lower, "insufficient cpu", "insufficient memory", "insufficient ephemeral-storage", "insufficient pods"):
		return &Failure{
			Name:        "FailedScheduling/InsufficientResources",
			Category:    CategoryScheduling,
			Summary:     "No node has enough capacity for the pod's resource requests",
			Remediation: "Reduce requests, add nodes, or enable cluster autoscaler.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
			Docs:        []string{"https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/"},
		}
	case strings.Contains(lower, "untolerated taint"), strings.Contains(lower, "had taint"):
		f := &Failure{
			Name:        "FailedScheduling/UntoleratedTaint",
			Category:    CategoryScheduling,
			Summary:     "Available nodes carry a taint the pod doesn't tolerate",
			Remediation: "Add a matching toleration or schedule onto a different node pool.",
			Object:      obj,
			Signals:     signals,
			Provider:    schedulingTaintProvider(lower),
		}
		return f
	case strings.Contains(lower, "node affinity"), strings.Contains(lower, "node selector"), strings.Contains(lower, "didn't match pod's node affinity/selector"):
		return &Failure{
			Name:        "FailedScheduling/NodeAffinity",
			Category:    CategoryScheduling,
			Summary:     "No node matches the pod's nodeAffinity / nodeSelector",
			Remediation: "Align node labels (e.g. topology.kubernetes.io/zone, kubernetes.io/arch) or relax the affinity to preferredDuringScheduling.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	case strings.Contains(lower, "didn't match pod affinity"), strings.Contains(lower, "didn't match pod anti-affinity"):
		return &Failure{
			Name:        "FailedScheduling/PodAffinity",
			Category:    CategoryScheduling,
			Summary:     "Pod (anti-)affinity cannot be satisfied by the cluster topology",
			Remediation: "Reduce the constraint or add capacity that satisfies it; consider preferredDuringScheduling.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	case strings.Contains(lower, "topology spread"):
		return &Failure{
			Name:        "FailedScheduling/TopologySpread",
			Category:    CategoryScheduling,
			Summary:     "Topology spread constraints cannot be satisfied",
			Remediation: "Loosen maxSkew, add capacity in the missing topology, or change to ScheduleAnyway.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	case strings.Contains(lower, "free ports"):
		return &Failure{
			Name:        "FailedScheduling/HostPortCollision",
			Category:    CategoryScheduling,
			Summary:     "No node has the requested hostPort free",
			Remediation: "Drop hostPort if not required, or pick a port not already bound on candidate nodes.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	case strings.Contains(lower, "too many pods"):
		return &Failure{
			Name:        "FailedScheduling/MaxPodsExceeded",
			Category:    CategoryScheduling,
			Summary:     "Candidate nodes are at their max-pods cap",
			Remediation: "Use larger instance types, raise kubelet --max-pods, or add nodes.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	case strings.Contains(lower, "insufficient nvidia.com/gpu"), strings.Contains(lower, "insufficient amd.com/gpu"):
		return &Failure{
			Name:        "FailedScheduling/ExtendedResource",
			Category:    CategoryScheduling,
			Summary:     "Requested extended resource (e.g. GPU) is not advertised on any node",
			Remediation: "Install the device plugin or add a node pool that exposes the resource.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	}

	return &Failure{
		Name:        "FailedScheduling",
		Category:    CategoryScheduling,
		Summary:     "Scheduler could not place the pod on any node",
		Remediation: "Inspect the scheduler message for the specific constraint that failed.",
		Object:      obj,
		Signals:     signals,
		Provider:    ProviderGeneric,
	}
}

// schedulingTaintProvider tags untolerated-taint failures by recognizing
// provider-specific taint keys in the scheduler message. (See §3.2.)
func schedulingTaintProvider(lowerMsg string) Provider {
	switch {
	case strings.Contains(lowerMsg, "cloud.google.com/gke-"), strings.Contains(lowerMsg, "components.gke.io/"):
		return ProviderGKE
	case strings.Contains(lowerMsg, "eks.amazonaws.com/"), strings.Contains(lowerMsg, "karpenter.sh/"):
		return ProviderEKS
	case strings.Contains(lowerMsg, "kubernetes.azure.com/"), strings.Contains(lowerMsg, "criticaladdonsonly"):
		return ProviderAKS
	}
	return ProviderGeneric
}
