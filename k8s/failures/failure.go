// Package failures classifies Kubernetes deployment failures into the canonical
// catalog defined in k8s/failure-modes.md. All classifiers are pure: they take
// typed snapshots (ContainerStatus, Pod, Event, Deployment) and return a
// structured Failure record. No API calls.
package failures

import (
	"time"
)

// Category is the top-level catalog section a Failure belongs to.
type Category string

const (
	CategoryImage      Category = "image"
	CategoryRuntime    Category = "runtime"
	CategoryScheduling Category = "scheduling"
	CategoryStorage    Category = "storage"
	CategoryNetwork    Category = "network"
	CategoryAdmission  Category = "admission"
	CategoryRollout    Category = "rollout"
	CategoryNode       Category = "node"
)

// Provider tags a Failure with the cloud provider it originates from.
// "generic" means the signal is platform-agnostic.
type Provider string

const (
	ProviderGeneric Provider = "generic"
	ProviderGKE     Provider = "gke"
	ProviderEKS     Provider = "eks"
	ProviderAKS     Provider = "aks"
)

// ObjectRef points at the K8s object the failure was observed on.
type ObjectRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Container string `json:"container,omitempty"`
}

// Signals carries the raw evidence that produced the classification.
// Consumers use it to render drill-down UIs without re-deriving the source.
type Signals struct {
	EventReason      string `json:"eventReason,omitempty"`
	EventMessage     string `json:"eventMessage,omitempty"`
	WaitingReason    string `json:"waitingReason,omitempty"`
	TerminatedReason string `json:"terminatedReason,omitempty"`
	ExitCode         *int32 `json:"exitCode,omitempty"`
	Condition        string `json:"condition,omitempty"`
}

// Failure is the structured output of every classifier; matches §12 of failure-modes.md.
type Failure struct {
	Name        string    `json:"name"`
	Category    Category  `json:"category"`
	Summary     string    `json:"summary"`
	Remediation string    `json:"remediation"`
	Object      ObjectRef `json:"object"`
	Signals     Signals   `json:"signals"`
	Provider    Provider  `json:"provider"`
	Docs        []string  `json:"docs,omitempty"`
	ObservedAt  time.Time `json:"observedAt,omitempty"`
}
