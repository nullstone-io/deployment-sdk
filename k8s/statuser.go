package k8s

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"k8s.io/api/apps/v1"
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

func (s Statuser) StatusOverview(ctx context.Context) (app.StatusOverviewResult, error) {
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
	replicaSetsResponse, err := client.AppsV1().ReplicaSets(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return so, fmt.Errorf("error retrieving app replica sets: %w", err)
	}
	replicaSets := ExcludeOldReplicaSets(replicaSetsResponse.Items)
	for _, replicaSet := range replicaSets {
		revision := AppStatusOverviewReplicaSetFromK8s(replicaSet)
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
	replicaSetsResponse, err := client.AppsV1().ReplicaSets(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app replica sets: %w", err)
	}
	svcsResponse, err := client.CoreV1().Services(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app services: %w", err)
	}
	podsResponse, err := client.CoreV1().Pods(s.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return st, fmt.Errorf("error retrieving app pods: %w", err)
	}
	statusPods := make(AppStatusPods, 0)
	for _, pod := range podsResponse.Items {
		statusPods = append(statusPods, AppStatusPodFromK8s(pod, svcsResponse.Items))
	}
	replicaSets := ExcludeOldReplicaSets(replicaSetsResponse.Items)
	for _, replicaSet := range replicaSets {
		revision := AppStatusReplicaSetFromK8s(replicaSet)
		revision.Pods = statusPods.ListByReplicaSet(revision.Name)
		st.ReplicaSets = append(st.ReplicaSets, revision)
	}

	return st, nil
}

// ExcludeOldReplicaSets filters out old replica sets
// Old replica sets have 0 replicas and aren't the newest deployment revision
func ExcludeOldReplicaSets(items []v1.ReplicaSet) []v1.ReplicaSet {
	maxRevision := 0
	for _, item := range items {
		if revision := RevisionFromReplicaSet(item); revision > maxRevision {
			maxRevision = revision
		}
	}

	result := make([]v1.ReplicaSet, 0)
	for _, item := range items {
		isNewestRevision := RevisionFromReplicaSet(item) == maxRevision
		if isNewestRevision || item.Status.Replicas > 0 {
			result = append(result, item)
		}
	}
	return result
}
