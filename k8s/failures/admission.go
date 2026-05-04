package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyAdmissionEvent handles §6 (Admission / Validation) when the trigger
// is an Event with reason=FailedCreate (typically on a ReplicaSet). The same
// message also surfaces on the Deployment as ReplicaFailure (handled in rollout.go).
func classifyAdmissionEvent(obj ObjectRef, ev corev1.Event) *Failure {
	if ev.Reason != "FailedCreate" {
		return nil
	}
	return classifyAdmissionMessage(obj, ev.Reason, ev.Message)
}

// classifyAdmissionMessage interprets an admission rejection message — used by
// both the Event path and the Deployment ReplicaFailure path so the catalog
// names match in either case.
func classifyAdmissionMessage(obj ObjectRef, reason, msg string) *Failure {
	lower := strings.ToLower(msg)
	signals := Signals{EventReason: reason, EventMessage: msg}

	switch {
	case strings.Contains(lower, "violates podsecurity"):
		return &Failure{
			Name:        "PodSecurityDenial",
			Category:    CategoryAdmission,
			Summary:     "Namespace's PodSecurity policy rejected the pod",
			Remediation: "Drop disallowed capabilities, set runAsNonRoot/seccompProfile, remove hostPath, or relax the namespace label.",
			Object:      obj, Signals: signals, Provider: ProviderGeneric,
			Docs: []string{"https://kubernetes.io/docs/concepts/security/pod-security-admission/"},
		}
	case strings.Contains(lower, "exceeded quota"):
		return &Failure{
			Name:        "ResourceQuotaExceeded",
			Category:    CategoryAdmission,
			Summary:     "Pod creation exceeded a ResourceQuota in the namespace",
			Remediation: "Raise the quota, reduce requests, or set defaults via LimitRange.",
			Object:      obj, Signals: signals, Provider: ProviderGeneric,
			Docs: []string{"https://kubernetes.io/docs/concepts/policy/resource-quotas/"},
		}
	case strings.Contains(lower, "limitrange"), strings.Contains(lower, "minimum cpu"), strings.Contains(lower, "maximum cpu usage"), strings.Contains(lower, "maximum memory usage"):
		return &Failure{
			Name:        "LimitRangeDenied",
			Category:    CategoryAdmission,
			Summary:     "Pod requests/limits violate the namespace LimitRange",
			Remediation: "Adjust requests/limits to fit the LimitRange bounds.",
			Object:      obj, Signals: signals, Provider: ProviderGeneric,
		}
	case strings.Contains(lower, "admission webhook"):
		return &Failure{
			Name:        "AdmissionWebhookDenied",
			Category:    CategoryAdmission,
			Summary:     "An admission webhook denied the request",
			Remediation: "Check the named webhook's policy/logs; if the webhook is unreachable, inspect failurePolicy.",
			Object:      obj, Signals: signals, Provider: admissionWebhookProvider(lower),
			Docs: []string{"https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/"},
		}
	case strings.Contains(lower, "autopilot.gke.io"), strings.Contains(lower, "gke warden"):
		return &Failure{
			Name:        "AutopilotPolicyDenied",
			Category:    CategoryAdmission,
			Summary:     "GKE Autopilot policy rejected the pod",
			Remediation: "Remove the disallowed feature (hostPath, hostNetwork, privileged, restricted capability) or use a Standard cluster.",
			Object:      obj, Signals: signals, Provider: ProviderGKE,
			Docs: []string{"https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-security-constraints"},
		}
	case strings.Contains(lower, "is being terminated"):
		return &Failure{
			Name:        "NamespaceTerminating",
			Category:    CategoryAdmission,
			Summary:     "Namespace is being terminated; new objects cannot be created",
			Remediation: "Wait for termination to complete, or investigate stuck finalizers on the namespace.",
			Object:      obj, Signals: signals, Provider: ProviderGeneric,
		}
	}
	return nil
}

// admissionWebhookProvider tags the webhook denial with a provider when the
// message names a known managed webhook (Azure Policy, GKE Policy Controller).
func admissionWebhookProvider(lowerMsg string) Provider {
	switch {
	case strings.Contains(lowerMsg, "azure-policy"), strings.Contains(lowerMsg, "k8sazure"):
		return ProviderAKS
	case strings.Contains(lowerMsg, "gatekeeper") && strings.Contains(lowerMsg, "azure"):
		return ProviderAKS
	}
	return ProviderGeneric
}
