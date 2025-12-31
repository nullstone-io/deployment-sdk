package logs

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

type StreamSource struct {
	Config     *rest.Config
	GetTimeout time.Duration
}

func (s StreamSource) GetStreamer(pod *corev1.Pod, containerName string, since time.Time, follow bool) (rest.ResponseWrapper, error) {
	sinceTime := metav1.NewTime(since)
	podLogOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
		SinceTime:  &sinceTime,
		Follow:     follow,
	}
	requests, err := polymorphichelpers.LogsForObjectFn(RestClientGetter{Config: s.Config}, pod, podLogOptions, s.GetTimeout, false)
	if err != nil {
		return nil, err
	}
	for _, request := range requests {
		return request, nil
	}
	return nil, nil
}
