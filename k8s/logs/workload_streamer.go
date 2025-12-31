package logs

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
)

var (
	getPodTimeout = 20 * time.Second
)

type NewConfiger func(ctx context.Context) (*rest.Config, error)

type WorkloadStreamer struct {
	Namespace    string
	WorkloadName string
	NewConfigFn  NewConfiger
	Options      app.LogStreamOptions
	Selector     string
}

func (s *WorkloadStreamer) Stream(ctx context.Context) error {
	buffer := &SimpleLogBuffer{Emitter: s.Options.Emitter}

	// Prepare a safe channel to signal
	drainCh := make(chan struct{})
	closeDrainCh := SafeClose(drainCh)

	// Start draining logs after closing this channel
	// It is signaled by a context cancellation
	defer closeDrainCh()
	go func() {
		<-ctx.Done()
		if s.Options.CancelFlushTimeout != nil && *s.Options.CancelFlushTimeout > 0 {
			// Wait for the cancel flush timeout before stopping
			time.Sleep(*s.Options.CancelFlushTimeout)
		}
		closeDrainCh()
	}()

	cfg, err := s.NewConfigFn(ctx)
	if err != nil {
		return s.newInitError("There was an error creating kubernetes client", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return s.newInitError("There was an error initializing kubernetes client", err)
	}

	podList, err := client.CoreV1().Pods(s.Namespace).List(ctx, metav1.ListOptions{LabelSelector: s.Selector})
	if err != nil {
		return s.newInitError("Failed to list pods", err)
	}

	streamers := &podStreamers{
		Namespace:    s.Namespace,
		WorkloadName: s.WorkloadName,
		LogSource: StreamSource{
			Config:     cfg,
			GetTimeout: getPodTimeout,
		},
	}
	// Add pods that already exist
	for _, pod := range podList.Items {
		s.debug(fmt.Sprintf("Adding pod (%s)...", pod.Name))
		streamers.Add(ctx, &pod, s.Options, buffer)
	}

	watcher, err := client.CoreV1().Pods(s.Namespace).Watch(ctx, metav1.ListOptions{LabelSelector: s.Selector, ResourceVersion: podList.ResourceVersion})
	if err != nil {
		return s.newInitError("Failed to watch pods", err)
	}

	// Watch pod events
	// When we get a new pod, start streaming
	// When a pod is removed, stop streaming (allow it to flush logs)
	// When a pod transitions to terminal (failed, succeeded), stop streaming (allow it to flush logs)
	go func(watcher watch.Interface) {
		defer watcher.Stop()

		for {
			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				switch event.Type {
				case watch.Added:
					s.debug(fmt.Sprintf("Adding pod (%s)...", pod.Name))
					streamers.Add(ctx, pod, s.Options, buffer)
				case watch.Modified:
					if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
						s.debug(fmt.Sprintf("Removing %s pod (%s)...", pod.Status.Phase, pod.Name))
						streamers.Remove(pod.Name)
					}
				case watch.Deleted:
					s.debug(fmt.Sprintf("Removing deleted pod (%s)...", pod.Name))
					streamers.Remove(pod.Name)
				}
			case <-drainCh:
				return
			}
		}
	}(watcher)

	// Wait for streamers to complete and flush logs
	streamers.Wait()
	s.debug("Streamer stopped.")

	return nil
}

func (s *WorkloadStreamer) newInitError(msg string, err error) app.LogInitError {
	return app.NewLogInitError("k8s", fmt.Sprintf("%s/%s", s.Namespace, s.WorkloadName), msg, err)
}

func (s *WorkloadStreamer) debug(msg string) {
	if s.Options.DebugLogger != nil {
		s.Options.DebugLogger.Printf("[DEBUG] %s\n", msg)
	}
}

type podStreamers struct {
	Namespace    string
	WorkloadName string
	LogSource    StreamGetter

	streamers map[string]*PodStreamer
	once      sync.Once
	mu        sync.Mutex
	wg        sync.WaitGroup
}

func (s *podStreamers) init() {
	s.once.Do(func() {
		s.streamers = map[string]*PodStreamer{}
	})
}

func (s *podStreamers) Wait() {
	s.wg.Wait()
}

func (s *podStreamers) Add(ctx context.Context, pod *corev1.Pod, options app.LogStreamOptions, buffer Buffer) {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if we're already streaming logs for this pod
	if _, exists := s.streamers[pod.Name]; exists {
		return
	}

	streamer := &PodStreamer{
		Namespace:    s.Namespace,
		WorkloadName: s.WorkloadName,
		Pod:          pod,
		LogSource:    s.LogSource,
	}
	s.streamers[pod.Name] = streamer
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		streamer.Stream(ctx, options, buffer)
	}()
}

func (s *podStreamers) Remove(podName string) {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()

	if streamer, exists := s.streamers[podName]; exists {
		streamer.Stop()
		delete(s.streamers, podName)
	}
}
