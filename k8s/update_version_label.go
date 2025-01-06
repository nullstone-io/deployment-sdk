package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	StandardVersionLabel = "app.kubernetes.io/version"
)

func UpdateVersionLabel(meta metav1.ObjectMeta, version string) metav1.ObjectMeta {
	meta.Labels[StandardVersionLabel] = version
	return meta
}
