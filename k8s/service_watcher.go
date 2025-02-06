package k8s

import (
	"context"
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

// ServiceWatcher monitors a k8s Service for the full deployment of an app to a Service
// This watcher streams relevant events (e.g. Pod "xyz-123" is serving, Pod "xyz-123" was removed)
//
// When a Deployment is created, a new ReplicaSet is created with a set of Pods
// As new pods transition to "Ready", they are added to the Endpoints object associated with the Service
// As soon as they are added to the Endpoints object, that Pod is considered "Ready"
// This streams changes to the Endpoints object (additions, removals)
type ServiceWatcher struct {
	Client           *kubernetes.Clientset
	ServiceNamespace string
	ServiceName      string
	OsWriters        logging.OsWriters

	stop chan struct{}
}

func NewServiceWatcher(client *kubernetes.Clientset, namespace, name string, osWriters logging.OsWriters) *ServiceWatcher {
	return &ServiceWatcher{
		Client:           client,
		ServiceNamespace: namespace,
		ServiceName:      name,
		OsWriters:        osWriters,
	}
}

func (w *ServiceWatcher) Stream() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_, stderr := w.OsWriters.Stdout(), w.OsWriters.Stderr()
		watcher, err := w.Client.CoreV1().Endpoints(w.ServiceNamespace).Watch(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", w.ServiceName).String(),
		})
		if err != nil {
			colorstring.Fprintln(stderr, DeployEvent{
				Timestamp: time.Now(),
				Type:      EventTypeError,
				Object:    "service-watcher",
				Message:   fmt.Sprintf("Failed to stream event for Service Endpoints: %s", err.Error()),
			}.String())
			return
		}
		defer watcher.Stop()

		prevState := EndpointsState{
			Ready:    make(map[string]corev1.EndpointAddress),
			NotReady: make(map[string]corev1.EndpointAddress),
		}
		for event := range watcher.ResultChan() {
			endpoints, ok := event.Object.(*corev1.Endpoints)
			if !ok {
				continue
			}
			if event.Type == watch.Deleted || event.Type == watch.Error {
				break
			}

			newState := EndpointsStateFromSubsets(endpoints.Subsets)
			w.emitDiff(prevState.Diff(newState))
			prevState = newState
		}
	}()
	return cancel
}

func (w *ServiceWatcher) emitDiff(diff EndpointsDiff) {
	stdout, _ := w.OsWriters.Stdout(), w.OsWriters.Stderr()
	now := time.Now()
	obj := fmt.Sprintf("endpoints/%s", w.ServiceName)
	diffEventString := func(eventType string, msg string) string {
		return DeployEvent{
			Timestamp: now,
			Type:      eventType,
			Object:    obj,
			Message:   msg,
		}.String()
	}
	identifier := func(ea corev1.EndpointAddress) string {
		if ea.TargetRef == nil {
			return "(unknown)"
		}
		return fmt.Sprintf("%s/%s", strings.ToLower(ea.TargetRef.Kind), ea.TargetRef.Name)
	}

	for _, ea := range diff.AddedToNotReady {
		colorstring.Fprintln(stdout, diffEventString(EventTypeNormal, fmt.Sprintf("%s was added to Service, not ready", identifier(ea))))
	}
	for _, ea := range diff.AddedToReady {
		colorstring.Fprintln(stdout, diffEventString(EventTypeNormal, fmt.Sprintf("%s was added to Service, ready", identifier(ea))))
	}
	for _, ea := range diff.Removed {
		colorstring.Fprintln(stdout, diffEventString(EventTypeNormal, fmt.Sprintf("%s was removed from Service", identifier(ea))))
	}
	for _, ea := range diff.DemotedToNotReady {
		colorstring.Fprintln(stdout, diffEventString(EventTypeWarning, fmt.Sprintf("%s transitioned from ready to not-ready", identifier(ea))))
	}
	for _, ea := range diff.PromotedToReady {
		colorstring.Fprintln(stdout, diffEventString(EventTypeNormal, fmt.Sprintf("%s transitioned to ready", identifier(ea))))
	}
}

func EndpointsStateFromSubsets(subsets []corev1.EndpointSubset) EndpointsState {
	state := EndpointsState{
		Ready:    make(map[string]corev1.EndpointAddress),
		NotReady: make(map[string]corev1.EndpointAddress),
	}
	for _, subset := range subsets {
		for _, address := range subset.Addresses {
			state.Ready[endpointKey(address)] = address
		}
		for _, address := range subset.NotReadyAddresses {
			state.NotReady[endpointKey(address)] = address
		}
	}
	return state
}

type EndpointsState struct {
	Ready    map[string]corev1.EndpointAddress
	NotReady map[string]corev1.EndpointAddress
}

func (a EndpointsState) All() map[string]corev1.EndpointAddress {
	result := make(map[string]corev1.EndpointAddress)
	for k, v := range a.Ready {
		result[k] = v
	}
	for k, v := range a.NotReady {
		result[k] = v
	}
	return result
}

type EndpointsDiff struct {
	Removed           map[string]corev1.EndpointAddress
	AddedToReady      map[string]corev1.EndpointAddress
	AddedToNotReady   map[string]corev1.EndpointAddress
	PromotedToReady   map[string]corev1.EndpointAddress
	DemotedToNotReady map[string]corev1.EndpointAddress
}

func (a EndpointsState) Diff(b EndpointsState) EndpointsDiff {
	prev := a.All()
	cur := b.All()

	diff := EndpointsDiff{
		Removed:           make(map[string]corev1.EndpointAddress),
		AddedToReady:      make(map[string]corev1.EndpointAddress),
		AddedToNotReady:   make(map[string]corev1.EndpointAddress),
		PromotedToReady:   make(map[string]corev1.EndpointAddress),
		DemotedToNotReady: make(map[string]corev1.EndpointAddress),
	}

	// Detect removed addresses
	for key := range prev {
		if ea, ok := cur[key]; !ok {
			diff.Removed[key] = ea
		}
	}

	// Detect newly added addresses
	for key, ea := range cur {
		if _, ok := prev[key]; !ok {
			if _, notReady := a.NotReady[key]; notReady {
				diff.AddedToNotReady[key] = ea
			} else {
				diff.AddedToReady[key] = ea
			}
		}
	}

	// Detect promoted from not ready to ready
	for key, ea := range a.NotReady {
		if _, ok := b.Ready[key]; ok {
			diff.PromotedToReady[key] = ea
		}
	}

	// Detect demoted from ready to not ready
	for key, ea := range a.Ready {
		if _, ok := b.NotReady[key]; ok {
			diff.DemotedToNotReady[key] = ea
		}
	}

	return diff
}

func endpointKey(address corev1.EndpointAddress) string {
	var nodeName string
	if address.NodeName != nil {
		nodeName = *address.NodeName
	}
	var refName string
	if address.TargetRef != nil {
		refName = address.TargetRef.Name
	}
	return fmt.Sprintf("%s/%s/%s", address.IP, nodeName, refName)
}
