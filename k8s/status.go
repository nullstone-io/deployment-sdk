package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"time"
)

type AppStatus struct {
	ReplicaSets []AppStatusReplicaSet `json:"replicaSets"`
}

type AppStatusReplicaSet struct {
	Name              string    `json:"name"`
	Revision          string    `json:"revision"`
	Generation        int64     `json:"generation"`
	CreatedAt         time.Time `json:"createdAt"`
	DesiredReplicas   int       `json:"desiredReplicas"`
	AvailableReplicas int       `json:"availableReplicas"`
	ReadyReplicas     int       `json:"readyReplicas"`
	Replicas          int       `json:"replicas"`

	Pods []AppStatusPod `json:"pods"`
}

func AppStatusReplicaSetFromK8s(rs appsv1.ReplicaSet) AppStatusReplicaSet {
	desired := 0
	if val, err := strconv.Atoi(rs.Annotations["deployment.kubernetes.io/desired-replicas"]); err == nil {
		desired = val
	}

	return AppStatusReplicaSet{
		Name:              rs.Name,
		Revision:          rs.Annotations["deployment.kubernetes.io/revision"],
		Generation:        rs.Status.ObservedGeneration,
		CreatedAt:         rs.CreationTimestamp.Time,
		DesiredReplicas:   desired,
		AvailableReplicas: int(rs.Status.AvailableReplicas),
		ReadyReplicas:     int(rs.Status.ReadyReplicas),
		Replicas:          int(rs.Status.Replicas),
		Pods:              make([]AppStatusPod, 0),
	}
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
	for _, cur := range pod.Spec.Containers {
		container := AppStatusContainerFromK8s(cur, findPodContainerStatus(pod, cur), svcs)
		containers = append(containers, container)
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
		Name:       pod.Name,
		CreatedAt:  pod.CreationTimestamp.Time,
		StartedAt:  startTime,
		ReplicaSet: findPodReplicaSet(pod),
		Phase:      string(pod.Status.Phase),
		Containers: containers,
		Conditions: conditions,
	}
}

type AppStatusPodContainer struct {
	Name    string                      `json:"name"`
	Ready   bool                        `json:"ready"`
	Started bool                        `json:"started"`
	Ports   []AppStatusPodContainerPort `json:"ports"`
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

	return AppStatusPodContainer{
		Name:    container.Name,
		Ready:   status.Ready,
		Started: status.Started != nil && *status.Started,
		Ports:   ports,
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
