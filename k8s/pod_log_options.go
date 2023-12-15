package k8s

import (
	"github.com/nullstone-io/deployment-sdk/app"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewPodLogOptions(lsOptions app.LogStreamOptions) *corev1.PodLogOptions {
	logOptions := &corev1.PodLogOptions{
		//Container:  o.Container,
		//Previous:   o.Previous,
		Timestamps: true,
	}

	if lsOptions.StartTime != nil {
		t := metav1.NewTime(*lsOptions.StartTime)
		logOptions.SinceTime = &t
	}

	if lsOptions.WatchInterval >= 0 {
		logOptions.Follow = true
	}

	return logOptions
}
