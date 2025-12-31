package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

var (
	containerNameFromRefSpecRegexp = regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`)
)

func NewPodLogStreamer(namespace, name, podName string, requests map[corev1.ObjectReference]rest.ResponseWrapper, cancelFlushTimeout, stopFlushTimeout time.Duration) *PodLogStreamer {
	return &PodLogStreamer{
		Namespace:          namespace,
		Name:               name,
		PodName:            podName,
		Requests:           requests,
		CancelFlushTimeout: cancelFlushTimeout,
		StopFlushTimeout:   stopFlushTimeout,
		stopCh:             make(chan struct{}),
	}
}

type PodLogStreamer struct {
	Namespace string
	Name      string
	PodName   string
	Requests  map[corev1.ObjectReference]rest.ResponseWrapper
	// CancelFlushTimeout provides a way to configure how long to wait when flushing logs after a cancellation
	// This occurs when the user cancels or when a runner is done
	CancelFlushTimeout time.Duration
	// StopFlushTimeout provides a way to configure how long to wait when flushing logs after a stop
	// This occurs when a pod stops
	StopFlushTimeout time.Duration

	IsDebugEnabled bool

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
	podName, containerName := s.parseRef(ref)

	readCloser, err := request.Stream(ctx)
	if err != nil {
		s.debug(fmt.Sprintf("Failed to open container logs for %s/%s: %s\n", podName, containerName, err))
		return
	}
	defer readCloser.Close()

	writer := buffer.NewWriter(fmt.Sprintf("%s/%s", podName, containerName))
	defer writer.Close()

	r := bufio.NewReader(readCloser)

	for {
		select {
		case <-ctx.Done():
			s.drainContainerLogs(podName, containerName, request, writer, s.CancelFlushTimeout)
			return
		case <-s.stopCh:
			s.drainContainerLogs(podName, containerName, request, writer, s.StopFlushTimeout)
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
				s.debug(fmt.Sprintf("Failed to read container logs for %s/%s: %s\n", podName, containerName, err))
				return
			}
		}
	}
}

// drainContainerLogs emits any remaining container logs
func (s *PodLogStreamer) drainContainerLogs(podName, containerName string, request rest.ResponseWrapper, writer LogBufferWriter, timeout time.Duration) {
	if timeout == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	readCloser, err := request.Stream(ctx)
	if err != nil {
		// ignore failed attempts to stream logs, we're done
		s.debug(fmt.Sprintf("Failed to open container logs during drain for %s/%s: %s\n", podName, containerName, err))
		return
	}
	defer readCloser.Close()
	r := bufio.NewReader(readCloser)

	// This loop terminates when the stream is no longer reachable *or* we timeout draining the logs
	for {
		str, readErr := r.ReadString('\n')
		if str != "" {
			str = strings.TrimSuffix(str, "\n")
			writer.Write(LogMessageFromLine(s.Namespace, s.Name, podName, containerName, str))
		}
		if readErr != nil {
			// any error stops draining the logs, including io.EOF error
			s.debug(fmt.Sprintf("Failed to read container logs during drain for %s/%s: %s\n", podName, containerName, err))
			return
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

func (s *PodLogStreamer) debug(msg string) {
	if s.IsDebugEnabled {
		fmt.Fprintf(os.Stderr, "[debug] %s", msg)
	}
}
