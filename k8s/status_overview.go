package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	"strconv"
	"time"
)

type AppStatusOverview struct {
	Revisions []AppStatusOverviewRevision `json:"revisions"`
}

type AppStatusOverviewRevision struct {
	Name              string    `json:"name"`
	Revision          string    `json:"revision"`
	Generation        int64     `json:"generation"`
	CreatedAt         time.Time `json:"createdAt"`
	DesiredReplicas   int       `json:"desiredReplicas"`
	AvailableReplicas int       `json:"availableReplicas"`
	ReadyReplicas     int       `json:"readyReplicas"`
	Replicas          int       `json:"replicas"`
}

func RevisionFromReplicaSet(rs appsv1.ReplicaSet) AppStatusOverviewRevision {
	desired := 0
	if val, err := strconv.Atoi(rs.Annotations["deployment.kubernetes.io/desired-replicas"]); err == nil {
		desired = val
	}

	return AppStatusOverviewRevision{
		Name:              rs.Name,
		Revision:          rs.Annotations["deployment.kubernetes.io/revision"],
		Generation:        rs.Status.ObservedGeneration,
		CreatedAt:         rs.CreationTimestamp.Time,
		DesiredReplicas:   desired,
		AvailableReplicas: int(rs.Status.AvailableReplicas),
		ReadyReplicas:     int(rs.Status.ReadyReplicas),
		Replicas:          int(rs.Status.Replicas),
	}
}
