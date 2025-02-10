package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"testing"
)

func TestCheckDeployment(t *testing.T) {
	//one := int32(1)
	two := int32(2)

	tests := []struct {
		name       string
		deployment v1.Deployment
		want       app.RolloutStatus
		err        error
	}{
		{
			name: "timed out",
			deployment: v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{RevisionAnnotation: "2"},
				},
				Spec: v1.DeploymentSpec{},
				Status: v1.DeploymentStatus{
					Conditions: []v1.DeploymentCondition{
						{
							Type:   v1.DeploymentAvailable,
							Reason: "SomethingElse",
						},
						{
							Type:   v1.DeploymentProgressing,
							Reason: deploymentutil.TimedOutReason,
						},
					},
				},
			},
			want: app.RolloutStatusFailed,
			err:  fmt.Errorf("deployment failed because of timeout (exceeding its deadline)"),
		},
		{
			name: "not fully available",
			deployment: v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{RevisionAnnotation: "2"},
				},
				Spec: v1.DeploymentSpec{
					Replicas: &two,
				},
				Status: v1.DeploymentStatus{
					UpdatedReplicas:   2,
					AvailableReplicas: 1,
				},
			},
			want: app.RolloutStatusInProgress,
		},
		{
			name: "completed",
			deployment: v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{RevisionAnnotation: "2"},
				},
				Spec: v1.DeploymentSpec{
					Replicas: &two,
				},
				Status: v1.DeploymentStatus{
					UpdatedReplicas:   2,
					AvailableReplicas: 2,
				},
			},
			want: app.RolloutStatusComplete,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CheckDeployment(&test.deployment)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.want, got)
		})
	}
}
