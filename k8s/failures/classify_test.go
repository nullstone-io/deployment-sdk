package failures

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// containerStatus is a tiny helper so each test case stays focused on the
// fields that drive classification.
func containerStatus(name string, waiting *corev1.ContainerStateWaiting, terminated *corev1.ContainerStateTerminated, lastTerminated *corev1.ContainerStateTerminated) corev1.ContainerStatus {
	cs := corev1.ContainerStatus{Name: name}
	cs.State.Waiting = waiting
	cs.State.Terminated = terminated
	if lastTerminated != nil {
		cs.LastTerminationState.Terminated = lastTerminated
	}
	return cs
}

func TestClassifyContainer_Image(t *testing.T) {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"}}

	cases := []struct {
		name         string
		waiting      *corev1.ContainerStateWaiting
		wantName     string
		wantCategory Category
		wantProvider Provider
	}{
		{
			name:         "ImagePullBackOff auth",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "Error response from daemon: unauthorized: authentication required"},
			wantName:     "ImagePullBackOff/Auth",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "ImagePullBackOff not-found",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ErrImagePull", Message: "manifest unknown for tag v1.2.3"},
			wantName:     "ImagePullBackOff/NotFound",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "ImagePullBackOff rate-limit",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "toomanyrequests: rate limit exceeded"},
			wantName:     "ImagePullBackOff/RateLimit",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "ImagePullBackOff network",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "no such host: registry.example.com"},
			wantName:     "ImagePullBackOff/Network",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "ECR auth tags eks",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "no basic auth credentials for 1234.dkr.ecr.us-east-1.amazonaws.com/myrepo"},
			wantName:     "ImagePullBackOff/Auth",
			wantCategory: CategoryImage,
			wantProvider: ProviderEKS,
		},
		{
			name:         "ACR auth tags aks",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "unauthorized: authentication required pulling from myregistry.azurecr.io"},
			wantName:     "ImagePullBackOff/Auth",
			wantCategory: CategoryImage,
			wantProvider: ProviderAKS,
		},
		{
			name:         "InvalidImageName",
			waiting:      &corev1.ContainerStateWaiting{Reason: "InvalidImageName", Message: "couldn't parse image reference"},
			wantName:     "InvalidImageName",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "ErrImageNeverPull",
			waiting:      &corev1.ContainerStateWaiting{Reason: "ErrImageNeverPull"},
			wantName:     "ErrImageNeverPull",
			wantCategory: CategoryImage,
			wantProvider: ProviderGeneric,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyContainer(pod, containerStatus("c", tc.waiting, nil, nil))
			require.NotNil(t, got)
			assert.Equal(t, tc.wantName, got.Name)
			assert.Equal(t, tc.wantCategory, got.Category)
			assert.Equal(t, tc.wantProvider, got.Provider)
			assert.Equal(t, "c", got.Object.Container)
		})
	}
}

func TestClassifyContainer_Runtime(t *testing.T) {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"}}

	t.Run("CrashLoopBackOff with OOMKilled previous", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c",
			&corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			nil,
			&corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137},
		))
		require.NotNil(t, got)
		assert.Equal(t, "CrashLoopBackOff/OOMKilled", got.Name)
		assert.Equal(t, CategoryRuntime, got.Category)
		assert.Equal(t, "CrashLoopBackOff", got.Signals.WaitingReason)
		assert.Equal(t, "OOMKilled", got.Signals.TerminatedReason)
		require.NotNil(t, got.Signals.ExitCode)
		assert.Equal(t, int32(137), *got.Signals.ExitCode)
	})

	t.Run("CrashLoopBackOff with exec format error → arch mismatch", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c",
			&corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			nil,
			&corev1.ContainerStateTerminated{Message: "standard_init_linux.go:228: exec user process caused: exec format error", ExitCode: 1},
		))
		require.NotNil(t, got)
		assert.Equal(t, "ImageArchitectureMismatch", got.Name)
		assert.Equal(t, CategoryImage, got.Category)
	})

	t.Run("CrashLoopBackOff with generic crash", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c",
			&corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			nil,
			&corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1},
		))
		require.NotNil(t, got)
		assert.Equal(t, "CrashLoopBackOff/AppCrash", got.Name)
		assert.Equal(t, CategoryRuntime, got.Category)
	})

	t.Run("CrashLoopBackOff without lastState falls back to plain CrashLoopBackOff", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c",
			&corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			nil, nil,
		))
		require.NotNil(t, got)
		assert.Equal(t, "CrashLoopBackOff", got.Name)
	})

	t.Run("CreateContainerConfigError", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c",
			&corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError", Message: `secret "db-creds" not found`},
			nil, nil,
		))
		require.NotNil(t, got)
		assert.Equal(t, "CreateContainerConfigError", got.Name)
		assert.Equal(t, CategoryRuntime, got.Category)
	})

	t.Run("Terminated OOMKilled (Job pod)", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c", nil,
			&corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}, nil,
		))
		require.NotNil(t, got)
		assert.Equal(t, "OOMKilled", got.Name)
	})

	t.Run("Terminated successfully → no failure", func(t *testing.T) {
		got := ClassifyContainer(pod, containerStatus("c", nil,
			&corev1.ContainerStateTerminated{Reason: "Completed", ExitCode: 0}, nil,
		))
		assert.Nil(t, got)
	})

	t.Run("Healthy running container → no failure", func(t *testing.T) {
		got := ClassifyContainer(pod, corev1.ContainerStatus{Name: "c", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}})
		assert.Nil(t, got)
	})
}

func TestClassifyPod_Scheduling(t *testing.T) {
	pendingPod := func(reason, msg string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: reason, Message: msg},
				},
			},
		}
	}

	cases := []struct {
		name         string
		pod          corev1.Pod
		wantName     string
		wantProvider Provider
	}{
		{
			name:         "insufficient cpu",
			pod:          pendingPod("Unschedulable", "0/3 nodes are available: 3 Insufficient cpu."),
			wantName:     "FailedScheduling/InsufficientResources",
			wantProvider: ProviderGeneric,
		},
		{
			name:         "untolerated taint generic",
			pod:          pendingPod("Unschedulable", "0/3 nodes are available: 3 node(s) had untolerated taint {dedicated: gpu}."),
			wantName:     "FailedScheduling/UntoleratedTaint",
			wantProvider: ProviderGeneric,
		},
		{
			name:         "untolerated taint EKS Karpenter",
			pod:          pendingPod("Unschedulable", "0/2 nodes are available: 2 node(s) had untolerated taint {karpenter.sh/disrupted: true}."),
			wantName:     "FailedScheduling/UntoleratedTaint",
			wantProvider: ProviderEKS,
		},
		{
			name:         "node affinity",
			pod:          pendingPod("Unschedulable", "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector."),
			wantName:     "FailedScheduling/NodeAffinity",
			wantProvider: ProviderGeneric,
		},
		{
			name:         "topology spread",
			pod:          pendingPod("Unschedulable", "0/3 nodes are available: 3 node(s) didn't match pod topology spread constraints."),
			wantName:     "FailedScheduling/TopologySpread",
			wantProvider: ProviderGeneric,
		},
		{
			name:         "scheduling gated",
			pod:          pendingPod("SchedulingGated", `pod has unresolved scheduling gates: ["example.com/gate"]`),
			wantName:     "SchedulingGated",
			wantProvider: ProviderGeneric,
		},
		{
			name:         "host port collision",
			pod:          pendingPod("Unschedulable", "0/3 nodes are available: 3 node(s) didn't have free ports for the requested pod ports."),
			wantName:     "FailedScheduling/HostPortCollision",
			wantProvider: ProviderGeneric,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyPod(tc.pod)
			require.Len(t, got, 1)
			assert.Equal(t, tc.wantName, got[0].Name)
			assert.Equal(t, CategoryScheduling, got[0].Category)
			assert.Equal(t, tc.wantProvider, got[0].Provider)
		})
	}
}

func TestClassifyPod_NodePressure(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"},
		Status: corev1.PodStatus{
			Phase:   corev1.PodFailed,
			Reason:  "Evicted",
			Message: "The node was low on resource: ephemeral-storage. Threshold quantity: 10Gi.",
		},
	}
	got := ClassifyPod(pod)
	require.Len(t, got, 1)
	assert.Equal(t, "Evicted/EphemeralStorage", got[0].Name)
	assert.Equal(t, CategoryNode, got[0].Category)
}

func TestClassifyEvent(t *testing.T) {
	makeEvent := func(reason, msg string) corev1.Event {
		return corev1.Event{
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "ns", Name: "p"},
			Reason:         reason,
			Message:        msg,
			LastTimestamp:  metav1.Time{Time: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		}
	}

	cases := []struct {
		name         string
		ev           corev1.Event
		wantNil      bool
		wantName     string
		wantCategory Category
		wantProvider Provider
	}{
		{
			name:         "FailedCreatePodSandBox EKS IPAM",
			ev:           makeEvent("FailedCreatePodSandBox", "failed to setup network: InsufficientFreeAddressesInSubnet: subnet-abc has no free addresses"),
			wantName:     "FailedCreatePodSandBox/IPExhaustion",
			wantCategory: CategoryNetwork,
			wantProvider: ProviderEKS,
		},
		{
			name:         "FailedCreatePodSandBox GKE alias range",
			ev:           makeEvent("FailedCreatePodSandBox", "IP_SPACE_EXHAUSTED on cluster pod range"),
			wantName:     "FailedCreatePodSandBox/IPExhaustion",
			wantCategory: CategoryNetwork,
			wantProvider: ProviderGKE,
		},
		{
			name:         "FailedCreatePodSandBox AKS subnet",
			ev:           makeEvent("FailedCreatePodSandBox", "Failed to allocate address: SubnetIsFull"),
			wantName:     "FailedCreatePodSandBox/IPExhaustion",
			wantCategory: CategoryNetwork,
			wantProvider: ProviderAKS,
		},
		{
			name:         "FailedMount multi-attach",
			ev:           makeEvent("FailedMount", "Multi-Attach error for volume pvc-xyz: Volume is already used by pod a/b"),
			wantName:     "FailedMount/MultiAttach",
			wantCategory: CategoryStorage,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "FailedAttachVolume EBS quota",
			ev:           makeEvent("FailedAttachVolume", "AttachVolume.Attach failed for volume: ebs.csi.aws.com VolumeLimitExceeded"),
			wantName:     "FailedAttachVolume/VolumeLimit",
			wantCategory: CategoryStorage,
			wantProvider: ProviderEKS,
		},
		{
			name:         "FailedCreate quota",
			ev:           makeEvent("FailedCreate", `Error creating: pods "x-abc" is forbidden: exceeded quota: compute-resources, requested: cpu=2, used: cpu=8, limited: cpu=10`),
			wantName:     "ResourceQuotaExceeded",
			wantCategory: CategoryAdmission,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "FailedCreate PSA",
			ev:           makeEvent("FailedCreate", `Error creating: pods "x-abc" is forbidden: violates PodSecurity "restricted:v1.28": ...`),
			wantName:     "PodSecurityDenial",
			wantCategory: CategoryAdmission,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "FailedCreate webhook",
			ev:           makeEvent("FailedCreate", `Error creating: admission webhook "policy.example.com" denied the request: blocked`),
			wantName:     "AdmissionWebhookDenied",
			wantCategory: CategoryAdmission,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "Unhealthy liveness probe",
			ev:           makeEvent("Unhealthy", "Liveness probe failed: HTTP probe failed with statuscode: 500"),
			wantName:     "LivenessProbeFailed",
			wantCategory: CategoryRuntime,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "Unhealthy readiness probe",
			ev:           makeEvent("Unhealthy", "Readiness probe failed: dial tcp: connect: connection refused"),
			wantName:     "ReadinessProbeFailed",
			wantCategory: CategoryRuntime,
			wantProvider: ProviderGeneric,
		},
		{
			name:         "FailedScheduling event reuses pending classifier",
			ev:           makeEvent("FailedScheduling", "0/3 nodes are available: 3 Insufficient memory."),
			wantName:     "FailedScheduling/InsufficientResources",
			wantCategory: CategoryScheduling,
			wantProvider: ProviderGeneric,
		},
		{
			name:    "Unknown reason returns nil",
			ev:      makeEvent("SomethingWeird", "..."),
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEvent(tc.ev)
			if tc.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tc.wantName, got.Name)
			assert.Equal(t, tc.wantCategory, got.Category)
			assert.Equal(t, tc.wantProvider, got.Provider)
			assert.Equal(t, tc.ev.LastTimestamp.Time, got.ObservedAt)
		})
	}
}

func TestClassifyDeployment(t *testing.T) {
	now := metav1.Time{Time: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)}

	t.Run("ProgressDeadlineExceeded", func(t *testing.T) {
		d := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "d"},
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
				Reason: "ProgressDeadlineExceeded", Message: "deployment exceeded deadline", LastTransitionTime: now,
			}}},
		}
		got := ClassifyDeployment(d)
		require.Len(t, got, 1)
		assert.Equal(t, "ProgressDeadlineExceeded", got[0].Name)
		assert.Equal(t, CategoryRollout, got[0].Category)
		assert.Equal(t, now.Time, got[0].ObservedAt)
	})

	t.Run("ReplicaFailure with quota message → ResourceQuotaExceeded", func(t *testing.T) {
		d := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "d"},
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue,
				Reason:  "FailedCreate",
				Message: `pods "x-abc" is forbidden: exceeded quota: compute-resources`,
			}}},
		}
		got := ClassifyDeployment(d)
		require.Len(t, got, 1)
		assert.Equal(t, "ResourceQuotaExceeded", got[0].Name)
		assert.Equal(t, CategoryAdmission, got[0].Category)
		assert.Equal(t, "ReplicaFailure=True:FailedCreate", got[0].Signals.Condition)
	})

	t.Run("ReplicaFailure with unrecognized message → generic", func(t *testing.T) {
		d := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "d"},
			Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue,
				Reason: "FailedCreate", Message: "something nobody has seen before",
			}}},
		}
		got := ClassifyDeployment(d)
		require.Len(t, got, 1)
		assert.Equal(t, "ReplicaFailure", got[0].Name)
		assert.Equal(t, CategoryRollout, got[0].Category)
	})

	t.Run("Healthy deployment → no failures", func(t *testing.T) {
		d := appsv1.Deployment{Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
			{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
		}}}
		assert.Empty(t, ClassifyDeployment(d))
	})
}
