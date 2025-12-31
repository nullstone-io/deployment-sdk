package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

var (
	getPodTimeout             = 20 * time.Second
	defaultCancelFlushTimeout = 0 * time.Millisecond
	defaultStopFlushTimeout   = 250 * time.Millisecond
)

func NewWorkloadLogStreamer(newConfigFn NewConfiger, options app.LogStreamOptions, namespace, name string) *WorkloadLogStreamer {
	selector := fmt.Sprintf("nullstone.io/app=%s", name)
	if options.Selector != nil && *options.Selector != "" {
		selector = *options.Selector
	}

	cancelFlushTimeout, stopFlushTimeout := defaultCancelFlushTimeout, defaultStopFlushTimeout
	if options.CancelFlushTimeout != nil {
		cancelFlushTimeout = *options.CancelFlushTimeout
	}
	if options.StopFlushTimeout != nil {
		stopFlushTimeout = *options.StopFlushTimeout
	}

	return &WorkloadLogStreamer{
		NewConfigFn:        newConfigFn,
		Selector:           selector,
		Emitter:            options.Emitter,
		PodLogOptions:      NewPodLogOptions(options),
		CancelFlushTimeout: cancelFlushTimeout,
		StopFlushTimeout:   stopFlushTimeout,
		Namespace:          namespace,
		Name:               name,

		streamers: map[string]*PodLogStreamer{},
	}
}

// WorkloadLogStreamer is responsible for streaming logs for a workload (StatefulSet, Deployment/ReplicaSet, DaemonSet, Job/CronJob)
// This will track the lifecycle pods as they enter/exit and start/stop streaming their logs
type WorkloadLogStreamer struct {
	NewConfigFn   NewConfiger
	Selector      string
	Emitter       app.LogEmitter
	PodLogOptions *corev1.PodLogOptions
	// CancelFlushTimeout provides a way to configure how long to wait when flushing logs after a cancellation
	// This occurs when the user cancels or when a runner is done
	CancelFlushTimeout time.Duration
	// StopFlushTimeout provides a way to configure how long to wait when flushing logs after a stop
	// This occurs when a pod stops
	StopFlushTimeout time.Duration
	Namespace        string
	Name             string

	IsDebugEnabled bool

	mu        sync.Mutex
	streamers map[string]*PodLogStreamer
}

// Stream begins streaming logs for all pods in a workload
// This watches for pods entering/exiting and starts/stops streaming logs for each
// This terminates when the input context is canceled
func (s *WorkloadLogStreamer) Stream(ctx context.Context) error {
	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return s.newInitError("There was an error creating kubernetes client", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return s.newInitError("There was an error initializing kubernetes client", err)
	}

	// Start pod watcher
	watcher, err := client.CoreV1().Pods(s.Namespace).Watch(ctx, metav1.ListOptions{LabelSelector: s.Selector})
	if err != nil {
		return s.newInitError("Failed to watch pods", err)
	}
	defer watcher.Stop()

	// Watch for change in pods
	// When pods are added, we start streaming logs immediately
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if pod.Status.Phase == corev1.PodRunning {
					s.addPod(ctx, pod, cfg)
				}
			case watch.Deleted:
				s.removePod(pod.Name)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *WorkloadLogStreamer) newInitError(msg string, err error) app.LogInitError {
	return app.NewLogInitError("k8s", fmt.Sprintf("%s/%s", s.Namespace, s.Name), msg, err)
}

func (s *WorkloadLogStreamer) addPod(ctx context.Context, pod *corev1.Pod, cfg *rest.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if we're already streaming logs for this pod
	if _, exists := s.streamers[pod.Name]; exists {
		return
	}

	requests, err := polymorphichelpers.LogsForObjectFn(RestClientGetter{Config: cfg}, pod, s.PodLogOptions, getPodTimeout, true)
	if err != nil || len(requests) == 0 {
		return
	}
	streamer := NewPodLogStreamer(s.Namespace, s.Name, pod.Name, requests, s.CancelFlushTimeout, s.StopFlushTimeout)
	streamer.IsDebugEnabled = s.IsDebugEnabled
	s.streamers[pod.Name] = streamer
	go streamer.Stream(ctx, &SimpleLogBuffer{Emitter: s.Emitter})
}

func (s *WorkloadLogStreamer) removePod(podName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if streamer, exists := s.streamers[podName]; exists {
		streamer.Stop()
		delete(s.streamers, podName)
	}
}
