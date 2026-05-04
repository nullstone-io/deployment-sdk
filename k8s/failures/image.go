package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyImage handles §1 (Image / Registry) when the trigger is a
// container's waiting state. Returns nil if the waiting reason isn't image-related.
func classifyImage(obj ObjectRef, status corev1.ContainerStatus) *Failure {
	w := status.State.Waiting
	if w == nil {
		return nil
	}
	switch w.Reason {
	case "ImagePullBackOff", "ErrImagePull":
		return imagePullFailure(obj, w.Reason, w.Message)
	case "InvalidImageName":
		return &Failure{
			Name:        "InvalidImageName",
			Category:    CategoryImage,
			Summary:     "Image reference is malformed",
			Remediation: "Fix the spec.containers[].image string (check for whitespace, unexpanded variables, or uppercase host).",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	case "ErrImageNeverPull":
		return &Failure{
			Name:        "ErrImageNeverPull",
			Category:    CategoryImage,
			Summary:     "imagePullPolicy=Never but the image isn't preloaded on the node",
			Remediation: "Set imagePullPolicy: IfNotPresent, or preload the image on the node.",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	case "ImageInspectError":
		return &Failure{
			Name:        "ImageInspectError",
			Category:    CategoryImage,
			Summary:     "Container runtime could not inspect the image",
			Remediation: "Re-pull the image or investigate node-local image corruption.",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	}
	return nil
}

// imagePullFailure sub-classifies ImagePullBackOff/ErrImagePull by parsing the message.
// Sub-reasons: auth, not-found, rate-limit, network. Defaults to a generic pull failure.
func imagePullFailure(obj ObjectRef, reason, msg string) *Failure {
	lower := strings.ToLower(msg)
	provider := imagePullProvider(lower)
	f := &Failure{
		Category: CategoryImage,
		Object:   obj,
		Signals:  Signals{WaitingReason: reason, EventMessage: msg},
		Provider: provider,
		Docs:     []string{"https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff"},
	}

	switch {
	case containsAny(lower, "unauthorized", "authentication required", "denied", "no basic auth credentials", "401"):
		f.Name = "ImagePullBackOff/Auth"
		f.Summary = "Registry rejected credentials (unauthorized)"
		f.Remediation = "Attach a valid imagePullSecret, refresh expired creds, or grant the cluster identity registry pull permission."
	case containsAny(lower, "manifest unknown", "not found", "repository does not exist", "name unknown"):
		f.Name = "ImagePullBackOff/NotFound"
		f.Summary = "Image tag or repository does not exist"
		f.Remediation = "Verify the image reference; ensure the tag is pushed."
	case containsAny(lower, "toomanyrequests", "rate limit", "429"):
		f.Name = "ImagePullBackOff/RateLimit"
		f.Summary = "Registry rate-limited the pull (commonly Docker Hub anonymous limits)"
		f.Remediation = "Authenticate to the registry, mirror the image, or use a registry without per-IP throttling."
	case containsAny(lower, "no such host", "i/o timeout", "connection refused", "dial tcp", "context deadline exceeded"):
		f.Name = "ImagePullBackOff/Network"
		f.Summary = "Could not reach the registry from the node"
		f.Remediation = "Check egress / DNS from the node; verify the registry hostname; private clusters may need NAT."
	default:
		f.Name = "ImagePullBackOff"
		f.Summary = "Container image could not be pulled"
		f.Remediation = "Inspect the registry message and verify the image exists and the cluster can authenticate."
	}
	return f
}

// imagePullProvider tags the failure with a provider when the message mentions
// a provider-specific registry host. Used for the §8.x/§9.6/§10.2 variants.
func imagePullProvider(lowerMsg string) Provider {
	switch {
	case strings.Contains(lowerMsg, ".dkr.ecr."):
		return ProviderEKS
	case strings.Contains(lowerMsg, ".azurecr.io"):
		return ProviderAKS
	case strings.Contains(lowerMsg, "gcr.io"), strings.Contains(lowerMsg, "pkg.dev"):
		return ProviderGKE
	}
	return ProviderGeneric
}
