package k8s

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
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
	AppName string
	Objects map[string]ObjectToTrack

	client     *dynamic.DynamicClient
	gvrBuilder *GroupVersionResourceBuilder
}

type ObjectToTrack struct {
	Object     v1.ObjectReference
	Resource   *unstructured.Unstructured
	IsTracking bool
}

func NewObjectTracker(appName string, client *dynamic.DynamicClient, disc *discovery.DiscoveryClient) *AppObjectsTracker {
	return &AppObjectsTracker{
		Objects:    make(map[string]ObjectToTrack),
		AppName:    appName,
		client:     client,
		gvrBuilder: &GroupVersionResourceBuilder{Client: disc},
	}
}

func (t *AppObjectsTracker) Load(ctx context.Context, object v1.ObjectReference) error {
	if _, ok := t.Objects[string(object.UID)]; ok {
		return nil
	}

	gvr, err := t.gvrBuilder.Build(object)
	if err != nil {
		return fmt.Errorf("error building group version resource: %w", err)
	}
	resource, err := t.client.Resource(gvr).Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
	if err != nil {
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
