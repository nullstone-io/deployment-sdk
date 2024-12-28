package k8s

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
		revision := RevisionFromReplicaSet(replicaSet)
		if revision.DesiredReplicas == 0 && revision.Replicas == 0 {
			// Don't show old revisions that have scaled down
			continue
		}
		so.Revisions = append(so.Revisions, revision)
	}
	return so, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	status := AppStatus{}

	return status, nil
}

type AppStatus struct {
}
