package k8s

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"strings"
)

type GroupVersionResourceBuilder struct {
	Client *discovery.DiscoveryClient
}

func (b *GroupVersionResourceBuilder) Build(object corev1.ObjectReference) (schema.GroupVersionResource, error) {
	// Parse the Group and Version from APIVersion
	group, version := parseGroupVersion(object.APIVersion)

	// Fetch all API resources
	resourceLists, err := b.Client.ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to fetch API resources: %v", err)
	}

	// Iterate through the resource lists to match Kind and GroupVersion
	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}

		// Match Group and Version
		if gv.Group == group && gv.Version == version {
			for _, resource := range resourceList.APIResources {
				if resource.Kind == object.Kind {
					// Return the matched GVR
					return schema.GroupVersionResource{
						Group:    group,
						Version:  version,
						Resource: resource.Name, // Resource name like "pods"
					}, nil
				}
			}
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("kind %s with APIVersion %s not found", object.Kind, object.APIVersion)
}

// parseGroupVersion splits APIVersion into Group and Version
func parseGroupVersion(apiVersion string) (string, string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		// Core API group has no group, only version
		return "", parts[0]
	}
	return parts[0], parts[1]
}
