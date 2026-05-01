package failures

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ClassifyDeployment surfaces §7 (Rollout) signals from a Deployment.
// Returns 0..N failures; in practice a Deployment can carry both
// ProgressDeadlineExceeded and ReplicaFailure simultaneously.
func ClassifyDeployment(d appsv1.Deployment) []Failure {
	obj := ObjectRef{Kind: "Deployment", Namespace: d.Namespace, Name: d.Name}
	var out []Failure
	for _, c := range d.Status.Conditions {
		switch c.Type {
		case appsv1.DeploymentProgressing:
			if c.Status == corev1.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
				out = append(out, Failure{
					Name:        "ProgressDeadlineExceeded",
					Category:    CategoryRollout,
					Summary:     "Deployment exceeded its progressDeadlineSeconds without completing",
					Remediation: "Drill into the newest ReplicaSet's pods to find the underlying failure (image, scheduling, probes, admission).",
					Object:      obj,
					Signals:     Signals{Condition: "Progressing=False:ProgressDeadlineExceeded", EventMessage: c.Message},
					Provider:    ProviderGeneric,
					Docs:        []string{"https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#failed-deployment"},
					ObservedAt:  c.LastTransitionTime.Time,
				})
			}
		case appsv1.DeploymentReplicaFailure:
			if c.Status == corev1.ConditionTrue {
				// ReplicaFailure messages are usually admission-shaped (quota / webhook /
				// PSA). Try the admission classifier first so the canonical name matches.
				if f := classifyAdmissionMessage(obj, c.Reason, c.Message); f != nil {
					f.Signals.Condition = "ReplicaFailure=True:" + c.Reason
					f.ObservedAt = c.LastTransitionTime.Time
					out = append(out, *f)
					continue
				}
				out = append(out, Failure{
					Name:        "ReplicaFailure",
					Category:    CategoryRollout,
					Summary:     "Deployment cannot create new pods",
					Remediation: "Inspect the condition message; common causes: quota, admission webhook, PSA, missing ServiceAccount.",
					Object:      obj,
					Signals:     Signals{Condition: "ReplicaFailure=True:" + c.Reason, EventMessage: c.Message},
					Provider:    ProviderGeneric,
					ObservedAt:  c.LastTransitionTime.Time,
				})
			}
		}
	}
	return out
}
