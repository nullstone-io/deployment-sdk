package k8s

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"time"
)

type Statuser struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger
}

func (s Statuser) StatusOverview(ctx context.Context) (any, error) {
	so := AppStatusOverview{Revisions: make([]AppStatusOverviewRevision, 0)}
	if s.AppName == "" {
		return so, nil
	}

	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return so, fmt.Errorf("error creating kubernetes client: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return so, fmt.Errorf("error initializing kubernetes client: %w", err)
	}

	appLabel := fmt.Sprintf("nullstone.io/app=%s", s.AppName)
	replicaSets, err := client.AppsV1().ReplicaSets(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return so, fmt.Errorf("error retrieving app replica sets: %w", err)
	}
	for _, replicaSet := range replicaSets.Items {
		so.Revisions = append(so.Revisions, RevisionFromReplicaSet(replicaSet))
	}
	return so, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	status := AppStatus{}

	return status, nil
}

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
}

func RevisionFromReplicaSet(rs v1.ReplicaSet) AppStatusOverviewRevision {
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
	}
}

type AppStatus struct {
}
