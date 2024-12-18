package k8s

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func FindDeploymentStartTime(ctx context.Context, client *kubernetes.Clientset, namespace string, deployment *appsv1.Deployment, revision string) *time.Time {
	replicaSets, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return nil
	}
	for _, rs := range replicaSets.Items {
		if rs.Annotations["deployment.kubernetes.io/revision"] == revision {
			t := rs.CreationTimestamp.Time
			return &t
		}
	}
	return nil
}
