package logs

import (
	"context"
	"sync"

	"github.com/nullstone-io/deployment-sdk/app"
	corev1 "k8s.io/api/core/v1"
)

type PodStreamer struct {
	Namespace    string
	WorkloadName string
	Pod          *corev1.Pod
	LogSource    StreamGetter

	once   sync.Once
	stopMu sync.Mutex
	stopCh chan struct{}
}

func (s *PodStreamer) init() {
	s.once.Do(func() {
		s.stopCh = make(chan struct{})
	})
}

func (s *PodStreamer) Stream(ctx context.Context, options app.LogStreamOptions, buffer Buffer) {
	s.init()
	defer s.Stop()
	defer s.debug(options, "Pod streamer stopped.")

	// Build a list of ContainerStreamers based on the Pod spec
	streamers := make([]*ContainerStreamer, 0)
	for _, containerName := range s.getContainerNames() {
		streamer := &ContainerStreamer{
			Namespace:     s.Namespace,
			WorkloadName:  s.WorkloadName,
			Pod:           s.Pod,
			ContainerName: containerName,
			LogSource:     s.LogSource,
		}
		streamers = append(streamers, streamer)
	}

	// Wait for Stop(), stop each ContainerStreamer
	go func() {
		<-s.stopCh
		for _, streamer := range streamers {
			streamer.Stop()
		}
	}()

	// Start each ContainerStreamer
	var wg sync.WaitGroup
	for _, streamer := range streamers {
		wg.Add(1)
		go func(streamer *ContainerStreamer) {
			defer wg.Done()
			streamer.Stream(ctx, options, buffer)
		}(streamer)
	}
	wg.Wait()
}

func (s *PodStreamer) Stop() {
	s.init()
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	select {
	case <-s.stopCh:
		// Channel is already close, ignore
	default:
		close(s.stopCh)
	}
}

func (s *PodStreamer) getContainerNames() []string {
	containerNames := make([]string, 0)
	for _, container := range s.Pod.Spec.InitContainers {
		containerNames = append(containerNames, container.Name)
	}
	for _, container := range s.Pod.Spec.Containers {
		containerNames = append(containerNames, container.Name)
	}
	return containerNames
}

func (s *PodStreamer) debug(options app.LogStreamOptions, msg string) {
	if options.DebugLogger != nil {
		options.DebugLogger.Printf("[DEBUG:%s] %s\n", s.Pod.Name, msg)
	}
}
