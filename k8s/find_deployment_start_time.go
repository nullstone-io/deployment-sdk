package k8s

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func FindDeploymentStartTime(ctx context.Context, client *kubernetes.Clientset, namespace string, deployment *appsv1.Deployment, revision int64) *time.Time {
	replicaSets, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return nil
	}
	revisionStr := fmt.Sprintf("%d", revision)
	for _, rs := range replicaSets.Items {
		if rs.Annotations[RevisionAnnotation] == revisionStr {
			t := rs.CreationTimestamp.Time
			return &t
		}
	}
	return nil
}
