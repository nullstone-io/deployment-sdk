package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s/failures"
	"github.com/nullstone-io/deployment-sdk/logging"
	"k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NewConfiger func(ctx context.Context) (*rest.Config, error)

// Statuser produces AppStatus and AppStatusOverview for a Nullstone app.
//
// Methods use pointer receivers so a single Statuser instance can cache the
// loaded k8s resources across calls — calling both Status and StatusOverview
// on the same instance reuses one set of API List requests.
type Statuser struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	Cluster      ClusterInfo
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger

	// Cache populated by initialize. Guarded by initOnce.
	initOnce    sync.Once
	initErr     error
	client      *kubernetes.Clientset
	replicaSets []v1.ReplicaSet
	services    []corev1.Service
	pods        []corev1.Pod
	jobs        []batchv1.Job
}

// initialize lazily fetches every k8s resource Status and StatusOverview need.
// It runs at most once per Statuser; the cached error (if any) is returned on
// subsequent calls so partial state doesn't get reused.
func (s *Statuser) initialize(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.loadResources(ctx)
	})
	return s.initErr
}

func (s *Statuser) loadResources(ctx context.Context) error {
	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("error initializing kubernetes client: %w", err)
	}
	s.client = client

	listOpts := metav1.ListOptions{LabelSelector: fmt.Sprintf("nullstone.io/app=%s", s.AppName)}

	rsResp, err := client.AppsV1().ReplicaSets(s.AppNamespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("error retrieving app replica sets: %w", err)
	}
	s.replicaSets = rsResp.Items

	svcResp, err := client.CoreV1().Services(s.AppNamespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("error retrieving app services: %w", err)
	}
	s.services = svcResp.Items

	podResp, err := client.CoreV1().Pods(s.AppNamespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("error retrieving app pods: %w", err)
	}
	s.pods = podResp.Items

	jobResp, err := client.BatchV1().Jobs(s.AppNamespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("error retrieving app jobs: %w", err)
	}
	s.jobs = jobResp.Items

	return nil
}

func (s *Statuser) StatusOverview(ctx context.Context) (app.StatusOverviewResult, error) {
	so := AppStatusOverview{
		Cluster:     s.Cluster,
		Namespace:   s.AppNamespace,
		ReplicaSets: make([]AppStatusOverviewReplicaSet, 0),
	}
	if s.AppName == "" {
		return so, nil
	}
	if err := s.initialize(ctx); err != nil {
		return so, err
	}

	so.DeploymentName = findDeploymentNameFromReplicaSets(s.replicaSets)
	so.Jobs = AppStatusJobSummaryFromK8s(s.jobs)
	for _, replicaSet := range ExcludeOldReplicaSets(s.replicaSets) {
		so.ReplicaSets = append(so.ReplicaSets, AppStatusOverviewReplicaSetFromK8s(replicaSet, s.services))
	}
	return so, nil
}

func (s *Statuser) Status(ctx context.Context) (any, error) {
	st := AppStatus{
		Cluster:     s.Cluster,
		Namespace:   s.AppNamespace,
		ReplicaSets: make([]AppStatusReplicaSet, 0),
		Jobs:        make([]AppStatusJobExecution, 0),
	}
	if s.AppName == "" {
		return st, nil
	}
	if err := s.initialize(ctx); err != nil {
		return st, err
	}

	st.DeploymentName = findDeploymentNameFromReplicaSets(s.replicaSets)

	statusPods := make(AppStatusPods, 0, len(s.pods))
	for _, pod := range s.pods {
		statusPods = append(statusPods, AppStatusPodFromK8s(pod, s.services))
	}
	for _, replicaSet := range ExcludeOldReplicaSets(s.replicaSets) {
		revision := AppStatusReplicaSetFromK8s(replicaSet, s.services)
		revision.Pods = statusPods.ListByReplicaSet(revision.Name)
		st.ReplicaSets = append(st.ReplicaSets, revision)
	}

	for _, job := range s.jobs {
		st.Jobs = append(st.Jobs, AppStatusJobExecutionFromK8s(job))
	}

	// Surface rollout-level failures (ProgressDeadlineExceeded / ReplicaFailure)
	// from the parent Deployment when one exists with the app name. A missing
	// Deployment isn't an error — some app types may not have one — so we ignore
	// NotFound and any transient classification miss.
	if dep, err := s.client.AppsV1().Deployments(s.AppNamespace).Get(ctx, s.AppName, metav1.GetOptions{}); err == nil && dep != nil {
		st.Failures = failures.ClassifyDeployment(*dep)
	} else if err != nil && !apierrors.IsNotFound(err) {
		return st, fmt.Errorf("error retrieving app deployment: %w", err)
	}

	return st, nil
}

// findDeploymentNameFromReplicaSets returns the name of the Deployment that
// owns one of the given ReplicaSets, or "" if none of them are Deployment-owned.
// Long-running services run under a Deployment; jobs and one-shot tasks don't,
// so an empty string is the correct signal that this app isn't a service.
func findDeploymentNameFromReplicaSets(rss []v1.ReplicaSet) string {
	for _, rs := range rss {
		for _, or := range rs.OwnerReferences {
			if or.Kind == "Deployment" {
				return or.Name
			}
		}
	}
	return ""
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
