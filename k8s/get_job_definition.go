package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetJobDefinition(ctx context.Context, client *kubernetes.Clientset, namespace string, name string) (*batchv1.Job, *v1.ConfigMap, error) {
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to retrieve job template: %w", err)
	} else if configMap == nil {
		return nil, nil, fmt.Errorf("job template (%s) does not exist in Kubernetes namespace (%s)", name, namespace)
	}

	// The job template is stored in the "template" field as a json blob
	var jobDef batchv1.Job
	val, ok := configMap.Data["template"]
	if !ok {
		return nil, nil, fmt.Errorf("job template (%s) contains no data for 'template' field", name)
	}
	if err := json.Unmarshal([]byte(val), &jobDef); err != nil {
		return nil, nil, fmt.Errorf("unable to parse job template: %w", err)
	}
	return &jobDef, configMap, nil
}

func UpdateJobDefinition(ctx context.Context, client *kubernetes.Clientset, namespace string, jobDef *batchv1.Job, configMap *v1.ConfigMap) error {
	raw, err := json.Marshal(jobDef)
	if err != nil {
		return fmt.Errorf("unable to serialize job definition to config map: %w", err)
	}
	configMap.Data["template"] = string(raw)
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}
