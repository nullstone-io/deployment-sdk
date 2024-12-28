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
	so := AppStatusOverview{ReplicaSets: make([]AppStatusOverviewReplicaSet, 0)}
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
		revision := AppStatusOverviewReplicaSetFromK8s(replicaSet)
		if revision.DesiredReplicas == 0 && revision.Replicas == 0 {
			// Don't show old revisions that have scaled down
			continue
		}
		so.ReplicaSets = append(so.ReplicaSets, revision)
	}
	return so, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	st := AppStatus{ReplicaSets: make([]AppStatusReplicaSet, 0)}
	if s.AppName == "" {
		return st, nil
	}

	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return st, fmt.Errorf("error creating kubernetes client: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return st, fmt.Errorf("error initializing kubernetes client: %w", err)
	}

	appLabel := fmt.Sprintf("nullstone.io/app=%s", s.AppName)
	replicaSets, err := client.AppsV1().ReplicaSets(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app replica sets: %w", err)
	}
	svcs, err := client.CoreV1().Services(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app services: %w", err)
	}
	pods, err := client.CoreV1().Pods(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app pods: %w", err)
	}
	statusPods := make(AppStatusPods, 0)
	for _, pod := range pods.Items {
		statusPods = append(statusPods, AppStatusPodFromK8s(pod, svcs.Items))
	}
	for _, replicaSet := range replicaSets.Items {
		revision := AppStatusReplicaSetFromK8s(replicaSet)
		if revision.DesiredReplicas == 0 && revision.Replicas == 0 {
			// Don't show old revisions that have scaled down
			continue
		}
		revision.Pods = statusPods.ListByReplicaSet(revision.Name)
		st.ReplicaSets = append(st.ReplicaSets, revision)
	}

	return st, nil
}
