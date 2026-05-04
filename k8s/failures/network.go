package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyNetworkEvent handles §5 (Network) when the trigger is an Event.
// Right now it covers FailedCreatePodSandBox (§5.1) — the highest-volume class —
// and SyncLoadBalancerFailed (§5.3).
func classifyNetworkEvent(obj ObjectRef, ev corev1.Event) *Failure {
	switch ev.Reason {
	case "FailedCreatePodSandBox":
		return sandboxFailure(obj, ev)
	case "SyncLoadBalancerFailed", "CreatingLoadBalancerFailed":
		return loadBalancerFailure(obj, ev)
	case "Unhealthy":
		// Probe failure events look like:
		//   "Liveness probe failed: ..." / "Readiness probe failed: ..." / "Startup probe failed: ..."
		// We label the probe type so the watcher / UI can render it accurately.
		return probeFailure(obj, ev)
	}
	return nil
}

func sandboxFailure(obj ObjectRef, ev corev1.Event) *Failure {
	lower := strings.ToLower(ev.Message)
	signals := Signals{EventReason: ev.Reason, EventMessage: ev.Message}

	switch {
	case strings.Contains(lower, "insufficientfreeaddressesinsubnet"):
		// InsufficientFreeAddressesInSubnet is the AWS VPC API error code surfaced
		// by the EKS VPC CNI; treat it as an EKS-specific signal.
		return &Failure{
			Name:        "FailedCreatePodSandBox/IPExhaustion",
			Category:    CategoryNetwork,
			Summary:     "VPC subnet has no free IP addresses",
			Remediation: "Enable prefix delegation, expand the subnet, or use custom networking.",
			Object:      obj, Signals: signals, Provider: ProviderEKS,
		}
	case strings.Contains(lower, "no ip addresses available"):
		return &Failure{
			Name:        "FailedCreatePodSandBox/IPExhaustion",
			Category:    CategoryNetwork,
			Summary:     "Pod CIDR / subnet has no free IP addresses",
			Remediation: "Expand the subnet/secondary range or migrate to overlay networking.",
			Object:      obj,
			Signals:     signals,
			Provider:    sandboxProvider(lower),
		}
	case strings.Contains(lower, "ip_space_exhausted"):
		return &Failure{
			Name:        "FailedCreatePodSandBox/IPExhaustion",
			Category:    CategoryNetwork,
			Summary:     "GKE pod IP space exhausted",
			Remediation: "Add secondary ranges or enable additional pod IP discovery.",
			Object:      obj, Signals: signals, Provider: ProviderGKE,
		}
	case strings.Contains(lower, "subnetisfull"):
		return &Failure{
			Name:        "FailedCreatePodSandBox/IPExhaustion",
			Category:    CategoryNetwork,
			Summary:     "Azure CNI subnet is full",
			Remediation: "Migrate to Azure CNI Overlay or expand the subnet.",
			Object:      obj, Signals: signals, Provider: ProviderAKS,
		}
	case strings.Contains(lower, "attachmentlimitexceeded"), strings.Contains(lower, "unable to attach eni"):
		return &Failure{
			Name:        "FailedCreatePodSandBox/ENILimit",
			Category:    CategoryNetwork,
			Summary:     "Node hit its ENI attachment limit",
			Remediation: "Use a larger instance type, enable prefix delegation, or reduce ENI usage per pod.",
			Object:      obj, Signals: signals, Provider: ProviderEKS,
		}
	case strings.Contains(lower, "context deadline exceeded"):
		return &Failure{
			Name:        "FailedCreatePodSandBox/CNITimeout",
			Category:    CategoryNetwork,
			Summary:     "CNI daemon was unresponsive while preparing the sandbox",
			Remediation: "Check the CNI agent (aws-node, azure-cns, netd) logs on the target node.",
			Object:      obj, Signals: signals, Provider: sandboxProvider(lower),
		}
	}
	return &Failure{
		Name:        "FailedCreatePodSandBox",
		Category:    CategoryNetwork,
		Summary:     "Container runtime / CNI could not create the pod sandbox",
		Remediation: "Inspect the CNI message; check node-level networking.",
		Object:      obj, Signals: signals, Provider: sandboxProvider(lower),
	}
}

func sandboxProvider(lowerMsg string) Provider {
	switch {
	case strings.Contains(lowerMsg, "vpc cni"), strings.Contains(lowerMsg, "aws-node"), strings.Contains(lowerMsg, "eni"):
		return ProviderEKS
	case strings.Contains(lowerMsg, "azure cni"), strings.Contains(lowerMsg, "subnetisfull"):
		return ProviderAKS
	case strings.Contains(lowerMsg, "ip_space_exhausted"), strings.Contains(lowerMsg, "alias"):
		return ProviderGKE
	}
	return ProviderGeneric
}

func loadBalancerFailure(obj ObjectRef, ev corev1.Event) *Failure {
	lower := strings.ToLower(ev.Message)
	signals := Signals{EventReason: ev.Reason, EventMessage: ev.Message}
	provider := loadBalancerProvider(lower)
	return &Failure{
		Name:        "LoadBalancerProvisioningFailed",
		Category:    CategoryNetwork,
		Summary:     "Cloud load balancer could not be provisioned",
		Remediation: "Inspect the cloud-controller message; common causes: missing IAM/role, missing subnet tags, exhausted public IP / LB quota.",
		Object:      obj, Signals: signals, Provider: provider,
	}
}

func loadBalancerProvider(lowerMsg string) Provider {
	switch {
	case strings.Contains(lowerMsg, "elb"), strings.Contains(lowerMsg, "no matching subnets"), strings.Contains(lowerMsg, "kubernetes.io/role/elb"):
		return ProviderEKS
	case strings.Contains(lowerMsg, "publicipcountlimitreached"), strings.Contains(lowerMsg, "outboundrulecannotbeused"):
		return ProviderAKS
	case strings.Contains(lowerMsg, "googleapi"), strings.Contains(lowerMsg, "forwardingrules"):
		return ProviderGKE
	}
	return ProviderGeneric
}

func probeFailure(obj ObjectRef, ev corev1.Event) *Failure {
	lower := strings.ToLower(ev.Message)
	probe := "Probe"
	switch {
	case strings.HasPrefix(lower, "liveness probe failed"):
		probe = "Liveness"
	case strings.HasPrefix(lower, "readiness probe failed"):
		probe = "Readiness"
	case strings.HasPrefix(lower, "startup probe failed"):
		probe = "Startup"
	}
	signals := Signals{EventReason: ev.Reason, EventMessage: ev.Message}
	remediation := "Verify the probe endpoint; widen initialDelaySeconds / failureThreshold or add a startupProbe."
	if probe == "Readiness" {
		remediation = "Verify the readiness endpoint; readiness keeps the pod out of Service Endpoints until it passes."
	}
	return &Failure{
		Name:        probe + "ProbeFailed",
		Category:    CategoryRuntime,
		Summary:     probe + " probe is failing for the container",
		Remediation: remediation,
		Object:      obj,
		Signals:     signals,
		Provider:    ProviderGeneric,
		Docs:        []string{"https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/"},
	}
}
