package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

var (
	containerNameFromRefSpecRegexp = regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`)
)

func NewPodLogStreamer(namespace, name, podName string, requests map[corev1.ObjectReference]rest.ResponseWrapper) *PodLogStreamer {
	return &PodLogStreamer{
		Namespace: namespace,
		Name:      name,
		PodName:   podName,
		Requests:  requests,
		stopCh:    make(chan struct{}),
	}
}

type PodLogStreamer struct {
	Namespace string
	Name      string
	PodName   string
	Requests  map[corev1.ObjectReference]rest.ResponseWrapper

	mu     sync.Mutex
	stopCh chan struct{}
}

func (s *PodLogStreamer) Stream(ctx context.Context, buffer LogBuffer) {
	defer s.Stop()

	var wg sync.WaitGroup

	// Start a goroutine for each container's log stream
	for ref, request := range s.Requests {
		wg.Add(1)
		go func(ref corev1.ObjectReference, request rest.ResponseWrapper) {
			defer wg.Done()
			s.streamContainerLogs(ctx, ref, request, buffer)
		}(ref, request)
	}

	// Wait for all log streams to complete
	wg.Wait()
}

// Stop stops streaming logs for this pod
func (s *PodLogStreamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stopCh:
		// Channel is already close, ignore
	default:
		close(s.stopCh)
	}
}

func (s *PodLogStreamer) streamContainerLogs(ctx context.Context, ref corev1.ObjectReference, request rest.ResponseWrapper, buffer LogBuffer) {
	readCloser, err := request.Stream(ctx)
	if err != nil {
		return
	}
	defer readCloser.Close()

	podName, containerName := s.parseRef(ref)

	writer := buffer.NewWriter(fmt.Sprintf("%s/%s", podName, containerName))
	defer writer.Close()

	r := bufio.NewReader(readCloser)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
			str, readErr := r.ReadString('\n')
			if str != "" {
				str = strings.TrimSuffix(str, "\n")
				writer.Write(LogMessageFromLine(s.Namespace, s.Name, s.PodName, containerName, str))
			}
			if readErr != nil {
				if readErr != io.EOF {
					// Log the error if needed
				}
				return
			}
		}
	}
}

func (s *PodLogStreamer) parseRef(ref corev1.ObjectReference) (string, string) {
	if ref.FieldPath == "" || ref.Name == "" {
		return ref.Name, ""
	}

	// We rely on ref.FieldPath to contain a reference to a container
	// including a container name (not an index) so we can get a container name
	// without making an extra API request.
	var containerName string
	containerNameMatches := containerNameFromRefSpecRegexp.FindStringSubmatch(ref.FieldPath)
	if len(containerNameMatches) == 2 {
		containerName = containerNameMatches[1]
	}

	return ref.Name, containerName
}
