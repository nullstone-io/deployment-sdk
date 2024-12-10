package k8s

import (
	"bufio"
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"golang.org/x/sync/errgroup"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"os"
	"regexp"
	"time"
)

var (
	getPodTimeout                  = 20 * time.Second
	maxFollowConcurrency           = 10
	containerNameFromRefSpecRegexp = regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`)
)

type NewConfiger func(ctx context.Context) (*rest.Config, error)

type LogStreamer struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger
}

func (l LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	if options.Emitter == nil {
		options.Emitter = app.NewWriterLogEmitter(os.Stdout)
	}

	appLabel := fmt.Sprintf("nullstone.io/app=%s", l.AppName)

	cfg, err := l.NewConfigFn(ctx)
	if err != nil {
		return l.newInitError("There was an error creating kubernetes client", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return l.newInitError("There was an error initializing kubernetes client", err)
	}
	pods, err := client.CoreV1().Pods(l.AppNamespace).List(ctx, metav1.ListOptions{LabelSelector: appLabel})
	if err != nil {
		return l.newInitError("There was an error looking for application pods", err)
	}
	if len(pods.Items) <= 0 {
		return l.newInitError("No pods were found for app in namespace", nil)
	}

	logOptions := NewPodLogOptions(options)
	requests, err := polymorphichelpers.LogsForObjectFn(RestClientGetter{Config: cfg}, pods, logOptions, getPodTimeout, true)
	if err != nil {
		return l.newInitError("There was an error initializing application log streamer", err)
	}

	if logOptions.Follow && len(requests) > 1 {
		if len(requests) > maxFollowConcurrency {
			tmpl := `You are attempting to follow %d log streams, exceeding the maximum allowed of %d. 
Restricting log streams to the first %d log streams.`
			fmt.Fprintf(l.OsWriters.Stderr(), tmpl,
				len(requests), maxFollowConcurrency, maxFollowConcurrency)
			newRequests := map[corev1.ObjectReference]rest.ResponseWrapper{}
			count := 0
			for k, v := range requests {
				newRequests[k] = v
				count++
				if count >= len(requests) {
					break
				}
			}
			requests = newRequests
		}
		return l.emitParallel(ctx, options.Emitter, requests)
	}
	return l.emitSequential(ctx, options.Emitter, requests)
}

func (l LogStreamer) emitParallel(ctx context.Context, emitter app.LogEmitter, requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	grp, ctx := errgroup.WithContext(ctx)
	for ref, request := range requests {
		grp.Go(l.writeRequest(ctx, emitter, ref, request))
	}
	return grp.Wait()
}

func (l LogStreamer) emitSequential(ctx context.Context, emitter app.LogEmitter, requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	for ref, request := range requests {
		if err := l.writeRequest(ctx, emitter, ref, request)(); err != nil {
			return err
		}
	}
	return nil
}

func (l LogStreamer) writeRequest(ctx context.Context, emitter app.LogEmitter, ref corev1.ObjectReference, request rest.ResponseWrapper) func() error {
	return func() error {
		readCloser, err := request.Stream(ctx)
		if err != nil {
			return l.newInitError("An error occurred streaming logs", err)
		}
		defer readCloser.Close()

		podName, containerName := l.parseRef(ref)

		r := bufio.NewReader(readCloser)
		for {
			str, readErr := r.ReadString('\n')
			if str != "" {
				emitter(LogMessageFromLine(l.AppNamespace, l.AppName, podName, containerName, str))
			}
			if readErr != nil {
				if readErr == io.EOF {
					return nil
				}
				return readErr
			}
		}
	}
}

func (l LogStreamer) parseRef(ref corev1.ObjectReference) (string, string) {
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

func (l LogStreamer) newInitError(msg string, err error) app.LogInitError {
	return app.NewLogInitError("k8s", fmt.Sprintf("%s/%s", l.AppNamespace, l.AppName), msg, err)
}
