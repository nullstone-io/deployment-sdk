package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyRuntime handles §2 (Runtime / Container Lifecycle).
// Pulls from both state.waiting (current state) and lastState.terminated
// (previous instance) so OOMKilled / app-crash variants of CrashLoopBackOff
// can be distinguished.
func classifyRuntime(obj ObjectRef, status corev1.ContainerStatus) *Failure {
	if f := classifyTerminated(obj, status); f != nil {
		return f
	}
	w := status.State.Waiting
	if w == nil {
		return nil
	}
	switch w.Reason {
	case "CrashLoopBackOff":
		return crashLoop(obj, status)
	case "CreateContainerConfigError":
		return &Failure{
			Name:        "CreateContainerConfigError",
			Category:    CategoryRuntime,
			Summary:     "Pod references a missing ConfigMap, Secret, or key",
			Remediation: "Create the referenced object/key, or mark the source `optional: true`.",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	case "CreateContainerError":
		return &Failure{
			Name:        "CreateContainerError",
			Category:    CategoryRuntime,
			Summary:     "Container runtime rejected the container spec",
			Remediation: "Inspect the runtime message — common causes: invalid mount, denied hostPath, missing seccomp profile, or post-crash name collision.",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	case "RunContainerError":
		return &Failure{
			Name:        "RunContainerError",
			Category:    CategoryRuntime,
			Summary:     "Container runtime failed to start the entrypoint",
			Remediation: "Verify the entrypoint exists and is executable in the image; check for arch mismatch.",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	case "ContainerCannotRun":
		return &Failure{
			Name:        "ContainerCannotRun",
			Category:    CategoryRuntime,
			Summary:     "Container runtime refused to run the container",
			Remediation: "Inspect the runtime message for specifics (entrypoint, capabilities, mount).",
			Object:      obj,
			Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
			Provider:    ProviderGeneric,
		}
	}
	return nil
}

// classifyTerminated handles the case where the *current* state is Terminated
// (typically a Job pod or a non-restarting container). It is also used during
// CrashLoopBackOff to interpret lastState.
func classifyTerminated(obj ObjectRef, status corev1.ContainerStatus) *Failure {
	t := status.State.Terminated
	if t == nil {
		return nil
	}
	if t.Reason == "Completed" || (t.ExitCode == 0 && t.Reason == "") {
		return nil
	}
	return terminationFailure(obj, *t, false)
}

// crashLoop interprets a container stuck in CrashLoopBackOff by looking at its
// previous (terminated) state. Without lastState, we can still report the loop
// itself; with lastState we can pinpoint OOMKilled vs app crash vs probe-killed.
func crashLoop(obj ObjectRef, status corev1.ContainerStatus) *Failure {
	w := status.State.Waiting // already non-nil per caller
	last := status.LastTerminationState.Terminated
	if last != nil {
		if f := terminationFailure(obj, *last, true); f != nil {
			f.Signals.WaitingReason = w.Reason
			return f
		}
	}
	return &Failure{
		Name:        "CrashLoopBackOff",
		Category:    CategoryRuntime,
		Summary:     "Container repeatedly exits and is being backed off",
		Remediation: "Inspect previous-container logs (kubectl logs --previous) and fix the startup path.",
		Object:      obj,
		Signals:     Signals{WaitingReason: w.Reason, EventMessage: w.Message},
		Provider:    ProviderGeneric,
		Docs:        []string{"https://kubernetes.io/docs/tasks/debug/debug-application/debug-pods/"},
	}
}

// terminationFailure classifies a single termination event (current Terminated or
// LastTerminationState). `inLoop` distinguishes a one-shot terminal from a CrashLoop ancestor.
func terminationFailure(obj ObjectRef, t corev1.ContainerStateTerminated, inLoop bool) *Failure {
	exit := t.ExitCode
	signals := Signals{TerminatedReason: t.Reason, ExitCode: &exit, EventMessage: t.Message}

	// OOMKilled is structurally distinct — kubelet sets reason directly.
	if t.Reason == "OOMKilled" {
		name := "OOMKilled"
		summary := "Container exceeded its memory limit and was killed by the kernel OOM killer"
		if inLoop {
			name = "CrashLoopBackOff/OOMKilled"
			summary = "Container repeatedly OOM-killed; loop continues"
		}
		return &Failure{
			Name:        name,
			Category:    CategoryRuntime,
			Summary:     summary,
			Remediation: "Raise resources.limits.memory, fix the leak, or size the runtime heap (GOMEMLIMIT, -XX:MaxRAMPercentage).",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
			Docs:        []string{"https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"},
		}
	}

	// Architecture mismatch: kubelet message often contains "exec format error".
	if strings.Contains(strings.ToLower(t.Message), "exec format error") {
		return &Failure{
			Name:        "ImageArchitectureMismatch",
			Category:    CategoryImage,
			Summary:     "Image architecture does not match the node (amd64 vs arm64)",
			Remediation: "Publish a multi-arch manifest (docker buildx --platform) or pin nodeSelector kubernetes.io/arch.",
			Object:      obj,
			Signals:     signals,
			Provider:    ProviderGeneric,
		}
	}

	// Generic crash. Exit code carries the signal in the loop case.
	name := "ContainerExited"
	summary := "Container exited unexpectedly"
	if inLoop {
		name = "CrashLoopBackOff/AppCrash"
		summary = "Container repeatedly crashes on startup"
	}
	switch exit {
	case 137:
		summary = "Container received SIGKILL (often a precursor to OOMKilled)"
	case 139:
		summary = "Container segfaulted (SIGSEGV)"
	case 143:
		summary = "Container received SIGTERM and did not exit gracefully"
	}
	return &Failure{
		Name:        name,
		Category:    CategoryRuntime,
		Summary:     summary,
		Remediation: "Check previous-container logs for the underlying exception or signal source.",
		Object:      obj,
		Signals:     signals,
		Provider:    ProviderGeneric,
	}
}
