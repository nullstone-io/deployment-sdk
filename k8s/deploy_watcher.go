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
	"strconv"
	"strings"
	"sync"
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
type DeployWatcher struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger
	Timeout      time.Duration

	client  *kubernetes.Clientset
	tracker *AppObjectsTracker
}

func (w *DeployWatcher) Watch(ctx context.Context, reference string, isFirstDeploy bool) error {
	stdout := w.OsWriters.Stdout()
	if reference == "" {
		fmt.Fprintln(stdout, "This deployment does not have to wait for any resource to become healthy.")
		return nil
	}
	if reference == DeployReferenceNoop {
		if isFirstDeploy {
			fmt.Fprintln(stdout, "Watching initial deployment.")
			reference = "1"
		} else {
			fmt.Fprintln(stdout, "This deployment did not cause any changes to the app. Skipping check for healthy.")
			return nil
		}
	}
	revision, err := strconv.ParseInt(reference, 10, 64)
	if err != nil {
		fmt.Fprintln(stdout, "Invalid deployment reference. Expected a deployment revision number.")
		return app.ErrFailed
	}
	if err := w.init(ctx); err != nil {
		return err
	}

	started := make(chan *time.Time)
	ended := make(chan struct{})
	flushed := make(chan struct{})
	go w.streamEvents(ctx, reference, started, ended, flushed)
	err := w.monitorDeployment(ctx, reference, started, ended)
	<-flushed
	return err
}

func (w *DeployWatcher) init(ctx context.Context) error {
	cfg, err := w.NewConfigFn(ctx)
	if err != nil {
		return w.newInitError("There was an error creating kubernetes client", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return w.newInitError("There was an error initializing kubernetes client", err)
	}
	w.client = client
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return w.newInitError("There was an error initializing kubernetes dynamic client", err)
	}
	discovery, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return w.newInitError("There was an error initializing kubernetes discovery client", err)
	}
	w.tracker = NewObjectTracker(w.AppName, dyn, discovery)
	return nil
}

func (w *DeployWatcher) newInitError(msg string, err error) app.LogInitError {
	return app.NewLogInitError("k8s", fmt.Sprintf("%s/%s", w.AppNamespace, w.AppName), msg, err)
}

// monitorDeployment polls Kubernetes for updates on the deployment
// This will run until the deployment completes, fails, or times out
func (w *DeployWatcher) monitorDeployment(ctx context.Context, revision int64, started chan *time.Time, ended chan struct{}) error {
	defer close(ended)
	defer close(started)

	timeout := watchDefaultTimeout
	if w.Timeout != 0 {
		timeout = w.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout := w.OsWriters.Stdout()
	init := sync.Once{}

	lastEventMsg := ""
	for {
		deployment, err := w.client.AppsV1().Deployments(w.AppNamespace).Get(ctx, w.AppName, metav1.GetOptions{})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return w.translateCancellation(ctx)
			}
			return fmt.Errorf("error retrieving deployment: %w", err)
		}
		if deployment != nil {
			init.Do(func() {
				start := FindDeploymentStartTime(ctx, w.client, w.AppNamespace, deployment, revision)
				if start != nil {
					colorstring.Fprintln(stdout, DeployEvent{
						Timestamp: *start,
						Type:      EventTypeNormal,
						Reason:    "Created",
						Object:    fmt.Sprintf("deployment/%s", w.AppName),
						Message:   fmt.Sprintf("Created deployment revision %d", revision),
					}.String())
				}
				started <- start
			})
		}

		evt, status, err := CheckDeployment(deployment, revision)
		if evt != nil {
			if msg := evt.String(); lastEventMsg != msg {
				colorstring.Fprintln(stdout, msg)
			}
		}
		if err != nil {
			return err
		}
		if status == app.RolloutStatusComplete {
			return nil
		}

		// Pause 3s between polling
		delay := 3 * time.Second
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return w.translateCancellation(ctx)
		}
	}
}

func (w *DeployWatcher) translateCancellation(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return app.ErrTimeout
		}
		return &app.CancelError{Reason: err.Error()}
	}
	return &app.CancelError{}
}

// streamEvents runs indefinitely and streams Kubernetes events associated with the app deployment
// This won't start streaming events until a deployment has started (expects message from `started` channel)
// If deployment never starts, this will flush events before completing
// Once started, events will be filtered appropriately and stream until `ended` channel is closed
func (w *DeployWatcher) streamEvents(ctx context.Context, started chan *time.Time, ended chan struct{}, flushed chan struct{}) {
	defer close(flushed)
	_, stderr := w.OsWriters.Stdout(), w.OsWriters.Stderr()
	earliest := time.Now()

	// Wait for initial fetch of deployment to acquire the start time of the deployment revision
	// If the deployment completes/cancels/fails, we're going to flush all events and quit
	// Otherwise, capture the revision creation time so that we can filter out previous events
	start, ok := <-started
	if start != nil {
		earliest = *start
	}
	w.emitAllEvents(earliest)
	if !ok {
		return
	}

	// Start watcher on all events in the namespace (there's no way to filter on just the events we want)
	watcher, err := w.client.CoreV1().Events(w.AppNamespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "There was an error watching events for app: %s\n", err)
		return
	}
	defer watcher.Stop()

	// Read events from watcher and translate those events into log messages on user stdout
	// If the deployment completes/cancels/fails
	for {
		select {
		case <-ended:
			return
		case ev := <-watcher.ResultChan():
			if event, ok := ev.Object.(*corev1.Event); ok && event != nil {
				w.emitEvent(ctx, earliest, *event)
			}
		}
	}
}

func (w *DeployWatcher) emitAllEvents(earliest time.Time) {
	ctx := context.Background()
	stderr := w.OsWriters.Stderr()
	timeoutSec := int64(2)
	opts := metav1.ListOptions{TimeoutSeconds: &timeoutSec}
	events, err := w.client.CoreV1().Events(w.AppNamespace).List(ctx, opts)
	if err != nil {
		fmt.Fprintf(stderr, "There was an error retrieving events for app: %s\n", err)
		return
	}
	for _, event := range events.Items {
		w.emitEvent(ctx, earliest, event)
	}
}

func (w *DeployWatcher) emitEvent(ctx context.Context, earliest time.Time, event corev1.Event) {
	stdout, stderr := w.OsWriters.Stdout(), w.OsWriters.Stderr()
	if event.LastTimestamp.Time.Before(earliest) {
		// Skip events that occurred before this deployment revision
		return
	}
	if err := w.tracker.Load(ctx, event.InvolvedObject); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fmt.Fprintf(stderr, "There was an error loading object for event: %s\n", err)
		return
	}
	if !w.tracker.IsTracking(event.InvolvedObject) {
		return
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
