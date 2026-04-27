package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type StreamGetter interface {
	GetStreamer(pod *corev1.Pod, containerName string, since time.Time, follow bool) (rest.ResponseWrapper, error)
}

// ContainerStreamer streams logs from a pod's container
// It is responsible for:
// - waiting for the container to start
// - streaming logs as app.LogMessage
// - flushing logs before stopping
type ContainerStreamer struct {
	Namespace     string
	WorkloadName  string
	Pod           *corev1.Pod
	ContainerName string
	LogSource     StreamGetter

	once   sync.Once
	stopMu sync.Mutex
	stopCh chan struct{}
}

func (s *ContainerStreamer) init() {
	s.once.Do(func() {
		s.stopCh = make(chan struct{})
	})
}

func (s *ContainerStreamer) Stream(ctx context.Context, options app.LogStreamOptions, buffer Buffer) {
	s.init()

	s.debug(options, "Starting log streamer...")
	defer s.debug(options, "Log streamer stopped.")

	since := time.Now()
	follow := false
	if options.StartTime != nil {
		since = *options.StartTime
	}
	if options.WatchInterval >= 0 {
		follow = true
	}
	request, err := s.LogSource.GetStreamer(s.Pod, s.ContainerName, since, follow)
	if err != nil {
		s.debug(options, fmt.Sprintf("Failed to initialize log streamer: %s\n", err))
		return
	}

	writer := buffer.NewWriter(fmt.Sprintf("%s/%s", s.Pod.Name, s.ContainerName))
	defer writer.Close()

	readCloser, err := s.startStream(ctx, request)
	if err != nil {
		s.debug(options, fmt.Sprintf("Failed to start stream: %s\n", err))
		return
	}
	defer readCloser.Close()
	r := bufio.NewReader(readCloser)

	for {
		select {
		case <-s.stopCh:
			s.debug(options, "Stopping log streamer...")
			readCloser.Close()
			s.flush(request, writer, options)
			return
		default:
			if readErr := s.writeLine(r, writer, options); readErr != nil {
				// The follow stream can EOF before the caller signals stop
				// (e.g. kubelet closes it on container exit). Flush here so
				// any bytes still buffered on the k8s side reach the writer.
				s.flush(request, writer, options)
				s.Stop()
				return
			}
		}
	}
}

func (s *ContainerStreamer) Stop() {
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

func (s *ContainerStreamer) startStream(ctx context.Context, request rest.ResponseWrapper) (io.ReadCloser, error) {
	for {
		stream, err := request.Stream(ctx)
		if err != nil {
			// Continue requesting log stream if ContainerCreating
			if strings.Contains(err.Error(), "ContainerCreating") {
				// Wait some time before retrying
				continue
			}
			return nil, err
		}
		return stream, nil
	}
}

// flushIdleWindow bounds how long flush() waits for more bytes after the last
// successful read. Lets callers set a generous StopFlushTimeout cap without
// penalising the fast path — a healthy pod's tail drains in a few ms.
const flushIdleWindow = 250 * time.Millisecond

func (s *ContainerStreamer) flush(request rest.ResponseWrapper, writer BufferWriter, options app.LogStreamOptions) {
	if options.StopFlushTimeout == nil || *options.StopFlushTimeout == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *options.StopFlushTimeout)
	defer cancel()

	readCloser, err := request.Stream(ctx)
	if err != nil {
		// ignore failed attempts to stream logs, we're done
		s.debug(options, fmt.Sprintf("Failed to open container logs during flush: %s", err))
		return
	}
	defer readCloser.Close()
	go func() {
		<-ctx.Done()
		readCloser.Close()
	}()

	r := bufio.NewReader(readCloser)

	// Run the read loop in a goroutine so the outer select can short-circuit
	// on an idle window even while ReadString is blocked.
	lineCh := make(chan struct{}, 1)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			if readErr := s.writeLine(r, writer, options); readErr != nil {
				return
			}
			select {
			case lineCh <- struct{}{}:
			default:
			}
		}
	}()

	idleTimer := time.NewTimer(flushIdleWindow)
	defer idleTimer.Stop()
	for {
		select {
		case <-doneCh:
			return
		case <-lineCh:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(flushIdleWindow)
		case <-idleTimer.C:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *ContainerStreamer) writeLine(r *bufio.Reader, writer BufferWriter, options app.LogStreamOptions) error {
	str, readErr := r.ReadString('\n')
	if str != "" {
		str = strings.TrimSuffix(str, "\n")
		writer.Write(MessageFromLine(s.Namespace, s.WorkloadName, s.Pod.Name, s.ContainerName, str))
	}
	if readErr != nil && readErr != io.EOF {
		s.debug(options, fmt.Sprintf("Failed to read container logs: %s", readErr))
	}
	return readErr
}

func (s *ContainerStreamer) debug(options app.LogStreamOptions, msg string) {
	if options.DebugLogger != nil {
		options.DebugLogger.Printf("[DEBUG:%s/%s] %s\n", s.Pod.Name, s.ContainerName, msg)
	}
}
