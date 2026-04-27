package k8s

import (
	"context"
	"fmt"

	"github.com/nullstone-io/deployment-sdk/logging"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// AppObjectsTracker enables tracking objects that belong to a Nullstone application
// This is done by reference labels["nullstone.io/app"] on the Kubernetes resource
// Each object is loaded once the first time Load() is called
// The resulting unstructured resource data is saved
type AppObjectsTracker struct {
	AppName   string
	Objects   map[string]ObjectToTrack
	OsWriters logging.OsWriters

	client        *dynamic.DynamicClient
	gvrBuilder    *GroupVersionResourceBuilder
	warnedUnknown map[string]bool
}

type ObjectToTrack struct {
	Object     v1.ObjectReference
	Resource   *unstructured.Unstructured
	IsTracking bool
}

func NewObjectTracker(appName string, client *dynamic.DynamicClient, disc *discovery.DiscoveryClient, osWriters logging.OsWriters) *AppObjectsTracker {
	return &AppObjectsTracker{
		Objects:       make(map[string]ObjectToTrack),
		AppName:       appName,
		OsWriters:     osWriters,
		client:        client,
		gvrBuilder:    &GroupVersionResourceBuilder{Client: disc},
		warnedUnknown: map[string]bool{},
	}
}

func (t *AppObjectsTracker) Load(ctx context.Context, object v1.ObjectReference) error {
	if _, ok := t.Objects[string(object.UID)]; ok {
		return nil
	}

	gvr, err := t.gvrBuilder.Build(object)
	if err != nil {
		// Unknown kind — skip rather than failing the watcher, but warn once per kind.
		key := object.APIVersion + "/" + object.Kind
		if t.OsWriters != nil && !t.warnedUnknown[key] {
			t.warnedUnknown[key] = true
			fmt.Fprintf(t.OsWriters.Stderr(), "Skipping events for unknown kind %s (%s): %s\n", object.Kind, object.APIVersion, err)
		}
		return nil
	}
	resource, err := t.client.Resource(gvr).Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error retrieving information about event object: %w", err)
	}
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	val, _ := labels["nullstone.io/app"]
	t.Objects[string(object.UID)] = ObjectToTrack{
		Object:     object,
		Resource:   resource,
		IsTracking: val == t.AppName,
	}
	return nil
}

func (t *AppObjectsTracker) IsTracking(object v1.ObjectReference) bool {
	existing, ok := t.Objects[string(object.UID)]
	return ok && existing.IsTracking
}
