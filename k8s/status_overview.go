package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	appsv1 "k8s.io/api/apps/v1"
	"strconv"
	"time"
)

var (
	_ app.StatusOverviewResult = &AppStatusOverview{}
)

type AppStatusOverview struct {
	ReplicaSets []AppStatusOverviewReplicaSet `json:"replicaSets"`
}

func (a AppStatusOverview) GetDeploymentVersions() []string {
	refs := make([]string, 0)
	for _, rs := range a.ReplicaSets {
		refs = append(refs, fmt.Sprintf("%d", rs.AppVersion))
	}
	return refs
}

type AppStatusOverviewReplicaSet struct {
	Name              string    `json:"name"`
	Revision          int       `json:"revision"`
	Generation        int64     `json:"generation"`
	AppVersion        string    `json:"appVersion"`
	CreatedAt         time.Time `json:"createdAt"`
	DesiredReplicas   int       `json:"desiredReplicas"`
	AvailableReplicas int       `json:"availableReplicas"`
	ReadyReplicas     int       `json:"readyReplicas"`
	Replicas          int       `json:"replicas"`
}

func AppStatusOverviewReplicaSetFromK8s(rs appsv1.ReplicaSet) AppStatusOverviewReplicaSet {
	desired := 0
	if val, err := strconv.Atoi(rs.Annotations["deployment.kubernetes.io/desired-replicas"]); err == nil {
		desired = val
	}

	return AppStatusOverviewReplicaSet{
		Name:              rs.Name,
		Revision:          RevisionFromReplicaSet(rs),
		Generation:        rs.Status.ObservedGeneration,
		CreatedAt:         rs.CreationTimestamp.Time,
		DesiredReplicas:   desired,
		AvailableReplicas: int(rs.Status.AvailableReplicas),
		ReadyReplicas:     int(rs.Status.ReadyReplicas),
		Replicas:          int(rs.Status.Replicas),
	}
}
