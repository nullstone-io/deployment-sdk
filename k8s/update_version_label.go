package k8s

import (
	"k8s.io/api/apps/v1"
)

const (
	StandardVersionLabel = "app.kubernetes.io/version"
)

func UpdateVersionLabel(deployment *v1.Deployment, version string) {
	if _, ok := deployment.ObjectMeta.Labels[StandardVersionLabel]; ok {
		deployment.ObjectMeta.Labels[StandardVersionLabel] = version
	}
}
