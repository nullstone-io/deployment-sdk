package k8s

import (
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
)

func GetContainerByName(deployment v1.Deployment, name string) (int, *core_v1.Container) {
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == name {
			return i, &container
		}
	}
	return -1, &core_v1.Container{}
}
