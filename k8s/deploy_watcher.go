package k8s

import (
	"context"
	"errors"
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

const (
	DeployReferenceNoop = "no-updated-revision"
)

var (
	_ app.DeployWatcher = &DeployWatcher{}

	watchDefaultTimeout = 15 * time.Minute
)

// DeployWatcher is responsible for watching a kubernetes deployment
// It detects completion/cancellation by watching the Deployment object
// While waiting, all events for the Deployment, Service, and Pods are logged
// It will log events and streams events and logs
type DeployWatcher struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger
	Timeout      time.Duration

	client        *kubernetes.Clientset
	tracker       *AppObjectsTracker
	deployStartCh chan *time.Time
}

func (s *DeployWatcher) Watch(ctx context.Context, reference string) error {
	stdout := s.OsWriters.Stdout()

	if reference == "" {
		fmt.Fprintln(stdout, "This deployment does not have to wait for any resource to become healthy.")
		return nil
	}
	if reference == DeployReferenceNoop {
		fmt.Fprintln(stdout, "This deployment did not cause any changes to the app. Skipping check for healthy.")
		return nil
	}
	if err := s.init(ctx); err != nil {
		return err
	}
	defer close(s.deployStartCh)

	timeout := watchDefaultTimeout
	if s.Timeout != 0 {
		timeout = s.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go s.streamEvents(ctx)()
	if err := s.watchDeployment(ctx, reference); err != nil {
		return err
	}
	return nil
}

func (s *DeployWatcher) init(ctx context.Context) error {
	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return s.newInitError("There was an error creating kubernetes client", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return s.newInitError("There was an error initializing kubernetes client", err)
	}
	s.client = client
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return s.newInitError("There was an error initializing kubernetes dynamic client", err)
	}
	discovery, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return s.newInitError("There was an error initializing kubernetes discovery client", err)
	}
	s.tracker = NewObjectTracker(s.AppName, dyn, discovery)
	s.deployStartCh = make(chan *time.Time)
	return nil
}

func (s *DeployWatcher) streamEvents(ctx context.Context) func() {
	stdout, stderr := s.OsWriters.Stdout(), s.OsWriters.Stderr()
	return func() {
		earliest := time.Now()
		// Wait for initial fetch of deployment to acquire the start time of the deployment revision
		select {
		case <-ctx.Done():
			return
		case start := <-s.deployStartCh:
			if start != nil {
				earliest = *start
			}
		}

		watcher, err := s.client.CoreV1().Events(s.AppNamespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fmt.Fprintf(stderr, "There was an error streaming events for app: %s\n", err)
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-watcher.ResultChan():
				if event, ok := ev.Object.(*corev1.Event); ok {
					if event.LastTimestamp.Time.Before(earliest) {
						// Skip events that occurred before this deployment revision
						continue
					}
					if err := s.tracker.Load(ctx, event.InvolvedObject); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						fmt.Fprintf(stderr, "There was an error loading object for event: %s\n", err)
						continue
					}
					if !s.tracker.IsTracking(event.InvolvedObject) {
						continue
					}
					obj := fmt.Sprintf("%s/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
					colorstring.Fprintln(stdout, DeployEvent{
						Timestamp: event.LastTimestamp.Time,
						Type:      event.Type,
						Reason:    event.Reason,
						Object:    obj,
						Message:   event.Message,
					}.String())
				}
			}
		}
	}
}

func (s *DeployWatcher) watchDeployment(ctx context.Context, reference string) error {
	appLabel := fmt.Sprintf("nullstone.io/app=%s", s.AppName)
	watcher, err := s.client.AppsV1().Deployments(s.AppNamespace).Watch(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return fmt.Errorf("error watching deployment: %w", err)
	}

	stdout := s.OsWriters.Stdout()
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return app.ErrTimeout
				}
				return &app.CancelError{Reason: err.Error()}
			}
			return &app.CancelError{}
		case ev := <-watcher.ResultChan():
			deployment, ok := ev.Object.(*appsv1.Deployment)
			if ok {
				ready, status := VerifyRevision(deployment, reference, stdout)
				if status == app.RolloutStatusFailed {
					return app.ErrFailed
				} else if ready {
					status, err := MapRolloutStatus(*deployment)
					if err != nil {
						return err
					} else if status == app.RolloutStatusComplete {
						return nil
					}
					s.deployStartCh <- FindDeploymentReplicaSet(ctx, s.client, s.AppNamespace, deployment, reference)
				}
			}
		}
	}
}

func (s *DeployWatcher) newInitError(msg string, err error) app.LogInitError {
	return app.NewLogInitError("k8s", fmt.Sprintf("%s/%s", s.AppNamespace, s.AppName), msg, err)
}
