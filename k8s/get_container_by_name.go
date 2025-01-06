package k8s

import (
	corev1 "k8s.io/api/core/v1"
)

func GetContainerByName(podTemplateSpec corev1.PodTemplateSpec, name string) (int, *corev1.Container) {
	for i, container := range podTemplateSpec.Spec.Containers {
		if container.Name == name {
			return i, &container
		}
	}
	return -1, &corev1.Container{}
}
