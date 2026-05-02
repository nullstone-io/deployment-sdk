package k8s

import (
	"strconv"
	"time"

	"github.com/nullstone-io/deployment-sdk/k8s/failures"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	PodTemplateHashLabel = "pod-template-hash"
)

// ClusterInfo identifies the cloud cluster an app is running on. Distinct from
// the ClusterInfoer interface (which produces kube-config Cluster details for
// auth) — this carries the human/cloud-provider identifiers used in status output.
type ClusterInfo struct {
	Region      string `json:"region"`
	ProjectId   string `json:"projectId,omitempty"`
	ClusterName string `json:"clusterName"`
}

type AppStatus struct {
	Cluster        ClusterInfo             `json:"cluster"`
	Namespace      string                  `json:"namespace"`
	DeploymentName string                  `json:"deploymentName"`
	ReplicaSets    []AppStatusReplicaSet   `json:"replicaSets"`
	Jobs           []AppStatusJobExecution `json:"jobs"`
	// Failures aggregates rollout-level failures (Deployment ProgressDeadlineExceeded,
	// ReplicaFailure conditions). Container and pod-level failures live on their
	// respective entries inside ReplicaSets.
	Failures []failures.Failure `json:"failures,omitempty"`
}

type AppStatusReplicaSet struct {
	Name              string                    `json:"name"`
	PodTemplateHash   string                    `json:"podTemplateHash"`
	Revision          int                       `json:"revision"`
	Generation        int64                     `json:"generation"`
	AppVersion        string                    `json:"appVersion"`
	CreatedAt         time.Time                 `json:"createdAt"`
	DesiredReplicas   int                       `json:"desiredReplicas"`
	AvailableReplicas int                       `json:"availableReplicas"`
	ReadyReplicas     int                       `json:"readyReplicas"`
	Replicas          int                       `json:"replicas"`
	Ports             []AppStatusReplicaSetPort `json:"ports"`

	Pods []AppStatusPod `json:"pods"`
}

func AppStatusReplicaSetFromK8s(rs appsv1.ReplicaSet, svcs []corev1.Service) AppStatusReplicaSet {
	desired := 0
	if val, err := strconv.Atoi(rs.Annotations["deployment.kubernetes.io/desired-replicas"]); err == nil {
		desired = val
	}

	return AppStatusReplicaSet{
		Name:              rs.Name,
		PodTemplateHash:   rs.Labels[PodTemplateHashLabel],
		Revision:          RevisionFromReplicaSet(rs),
		Generation:        rs.Status.ObservedGeneration,
		AppVersion:        rs.Labels[StandardVersionLabel],
		CreatedAt:         rs.CreationTimestamp.Time,
		DesiredReplicas:   desired,
		AvailableReplicas: int(rs.Status.AvailableReplicas),
		ReadyReplicas:     int(rs.Status.ReadyReplicas),
		Replicas:          int(rs.Status.Replicas),
		Ports:             AggregateReplicaSetPorts(rs, svcs),
		Pods:              make([]AppStatusPod, 0),
	}
}

type AppStatusReplicaSetPort struct {
	Protocol      string `json:"protocol"`
	HostPort      int    `json:"hostPort"`
	ContainerName string `json:"containerName"`
	ContainerPort int    `json:"containerPort"`
}

// AggregateReplicaSetPorts derives the host->container port mappings for a replica set
// by matching the pod template's container ports against the target ports of the given services.
func AggregateReplicaSetPorts(rs appsv1.ReplicaSet, svcs []corev1.Service) []AppStatusReplicaSetPort {
	ports := make([]AppStatusReplicaSetPort, 0)
	for _, container := range rs.Spec.Template.Spec.Containers {
		for _, svc := range svcs {
			for _, port := range svc.Spec.Ports {
				svcPort := port.TargetPort.IntValue()
				for _, cport := range container.Ports {
					if int(cport.ContainerPort) == svcPort {
						ports = append(ports, AppStatusReplicaSetPort{
							Protocol:      string(cport.Protocol),
							HostPort:      int(port.Port),
							ContainerName: container.Name,
							ContainerPort: int(cport.ContainerPort),
						})
					}
				}
			}
		}
	}
	return ports
}

type AppStatusPods []AppStatusPod

func (s AppStatusPods) ListByReplicaSet(name string) []AppStatusPod {
	result := make([]AppStatusPod, 0)
	for _, pod := range s {
		if pod.ReplicaSet == name {
			result = append(result, pod)
		}
	}
	return result
}

type AppStatusPod struct {
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt"`
	ReplicaSet string     `json:"replicaSet"`
	// Phase represents the current lifecycle of the pod
	// Available: Pending, Running, Succeeded, Failed
	Phase      string                  `json:"phase"`
	Conditions []AppStatusPodCondition `json:"conditions"`
	Containers []AppStatusPodContainer `json:"containers"`
	// MaxRestartCount is the highest RestartCount across the pod's containers.
	MaxRestartCount int `json:"maxRestartCount"`
	// LastRestartedAt is the LastRestartedAt of the container with MaxRestartCount, or nil if no container has restarted.
	LastRestartedAt *time.Time `json:"lastRestartedAt"`
	// Failures aggregates pod-level classification (scheduling, eviction).
	// Container-level classification lives on each AppStatusPodContainer.Failure.
	Failures []failures.Failure `json:"failures,omitempty"`
}

type AppStatusPodCondition struct {
	// Type refers to the condition type
	// Available: ContainersReady, Initialized, Ready, PodScheduled, DisruptionTarget
	Type               string    `json:"type"`
	Status             *bool     `json:"status"`
	Message            string    `json:"message"`
	LastProbeTime      time.Time `json:"lastProbeTime"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

func AppStatusPodFromK8s(pod corev1.Pod, svcs []corev1.Service) AppStatusPod {
	containers := make([]AppStatusPodContainer, 0)
	maxRestartCount := 0
	var lastRestartedAt *time.Time
	for _, cur := range pod.Spec.Containers {
		status := findPodContainerStatus(pod, cur)
		container := AppStatusContainerFromK8s(cur, status, svcs)
		if status != nil {
			container.Failure = failures.ClassifyContainer(pod, *status)
		}
		containers = append(containers, container)
		if container.RestartCount > maxRestartCount {
			maxRestartCount = container.RestartCount
			lastRestartedAt = container.LastRestartedAt
		}
	}

	var startTime *time.Time
	if pod.Status.StartTime != nil {
		startTime = &pod.Status.StartTime.Time
	}

	conditions := make([]AppStatusPodCondition, 0)
	for _, cur := range pod.Status.Conditions {
		condition := AppStatusPodCondition{
			Type:               string(cur.Type),
			Status:             nil,
			Message:            cur.Message,
			LastProbeTime:      cur.LastProbeTime.Time,
			LastTransitionTime: cur.LastTransitionTime.Time,
		}
		if cur.Status != corev1.ConditionUnknown {
			status := cur.Status == corev1.ConditionTrue
			condition.Status = &status
		}
		conditions = append(conditions, condition)
	}

	return AppStatusPod{
		Name:            pod.Name,
		CreatedAt:       pod.CreationTimestamp.Time,
		StartedAt:       startTime,
		ReplicaSet:      findPodReplicaSet(pod),
		Phase:           string(pod.Status.Phase),
		Containers:      containers,
		Conditions:      conditions,
		MaxRestartCount: maxRestartCount,
		LastRestartedAt: lastRestartedAt,
		Failures:        failures.ClassifyPod(pod),
	}
}

type AppStatusPodContainer struct {
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Command      []string `json:"command"`
	Ready        bool     `json:"ready"`
	Started      bool     `json:"started"`
	RestartCount int      `json:"restartCount"`
	// LastRestartedAt is when the most recent restart occurred (the previous instance's termination time),
	// or nil if the container has never restarted.
	LastRestartedAt *time.Time                  `json:"lastRestartedAt"`
	Ports           []AppStatusPodContainerPort `json:"ports"`
	// Failure is the classified failure for this container, if any.
	// Populated by failures.ClassifyContainer; nil when healthy.
	Failure *failures.Failure `json:"failure,omitempty"`
}

func AppStatusContainerFromK8s(container corev1.Container, status *corev1.ContainerStatus, svcs []corev1.Service) AppStatusPodContainer {
	ports := make([]AppStatusPodContainerPort, 0)
	for _, svc := range svcs {
		ips := append(svc.Spec.ClusterIPs, svc.Spec.ExternalIPs...)
		for _, port := range svc.Spec.Ports {
			svcPort := port.TargetPort.IntValue()
			for _, cport := range container.Ports {
				if int(cport.ContainerPort) == svcPort {
					ports = append(ports, AppStatusPodContainerPort{
						Protocol:      string(cport.Protocol),
						IpAddresses:   ips,
						ContainerPort: int(cport.ContainerPort),
						HostPort:      int(port.Port),
					})
				}
			}
		}
	}

	var ready bool
	var started bool
	var restartCount int
	var lastRestartedAt *time.Time
	if status != nil {
		ready = status.Ready
		started = status.Started != nil && *status.Started
		restartCount = int(status.RestartCount)
		if restartCount > 0 {
			if term := status.LastTerminationState.Terminated; term != nil && !term.FinishedAt.IsZero() {
				t := term.FinishedAt.Time
				lastRestartedAt = &t
			} else if running := status.State.Running; running != nil && !running.StartedAt.IsZero() {
				t := running.StartedAt.Time
				lastRestartedAt = &t
			}
		}
	}

	return AppStatusPodContainer{
		Name:            container.Name,
		Image:           container.Image,
		Command:         container.Command,
		Ready:           ready,
		Started:         started,
		RestartCount:    restartCount,
		LastRestartedAt: lastRestartedAt,
		Ports:           ports,
	}
}

type AppStatusPodContainerPort struct {
	Protocol      string   `json:"protocol"`
	IpAddresses   []string `json:"ipAddresses"`
	ContainerPort int      `json:"containerPort"`
	HostPort      int      `json:"hostPort"`
}

func findPodReplicaSet(pod corev1.Pod) string {
	for _, or := range pod.OwnerReferences {
		if or.Kind == "ReplicaSet" {
			return or.Name
		}
	}
	return ""
}

func findPodContainerStatus(pod corev1.Pod, container corev1.Container) *corev1.ContainerStatus {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container.Name {
			return &status
		}
	}
	return nil
}
