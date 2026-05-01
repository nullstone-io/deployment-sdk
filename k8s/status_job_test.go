package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeJob(name string, opts func(*batchv1.Job)) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID("uid-" + name),
		},
	}
	if opts != nil {
		opts(&job)
	}
	return job
}

func TestJobPhase(t *testing.T) {
	cases := []struct {
		name string
		job  batchv1.Job
		want AppStatusJobExecutionPhase
	}{
		{
			name: "queued (no startTime, no active)",
			job:  makeJob("j", nil),
			want: JobPhaseQueued,
		},
		{
			name: "running via active counter",
			job: makeJob("j", func(j *batchv1.Job) {
				j.Status.Active = 1
			}),
			want: JobPhaseRunning,
		},
		{
			name: "running via startTime even with no active",
			job: makeJob("j", func(j *batchv1.Job) {
				now := metav1.Now()
				j.Status.StartTime = &now
			}),
			want: JobPhaseRunning,
		},
		{
			name: "succeeded via Complete=True",
			job: makeJob("j", func(j *batchv1.Job) {
				j.Status.Conditions = []batchv1.JobCondition{
					{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				}
			}),
			want: JobPhaseSucceeded,
		},
		{
			name: "failed via Failed=True wins over active",
			job: makeJob("j", func(j *batchv1.Job) {
				j.Status.Active = 1
				j.Status.Conditions = []batchv1.JobCondition{
					{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
				}
			}),
			want: JobPhaseFailed,
		},
		{
			name: "Complete condition with Status=False is not Succeeded",
			job: makeJob("j", func(j *batchv1.Job) {
				j.Status.Conditions = []batchv1.JobCondition{
					{Type: batchv1.JobComplete, Status: corev1.ConditionFalse},
				}
			}),
			want: JobPhaseQueued,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, jobPhase(tc.job))
		})
	}
}

func TestAppStatusJobExecutionFromK8s(t *testing.T) {
	created := metav1.Time{Time: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)}
	started := metav1.Time{Time: time.Date(2026, 5, 1, 12, 0, 5, 0, time.UTC)}
	completed := metav1.Time{Time: time.Date(2026, 5, 1, 12, 1, 0, 0, time.UTC)}

	t.Run("succeeded carries timestamps", func(t *testing.T) {
		job := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "nightly-sync",
				UID:               types.UID("uid-1"),
				CreationTimestamp: created,
				Labels:            map[string]string{StandardVersionLabel: "v1.2.3"},
			},
			Status: batchv1.JobStatus{
				StartTime:      &started,
				CompletionTime: &completed,
				Conditions: []batchv1.JobCondition{
					{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				},
			},
		}
		got := AppStatusJobExecutionFromK8s(job)
		assert.Equal(t, "nightly-sync", got.Name)
		assert.Equal(t, "uid-1", got.ExecutionId)
		assert.Equal(t, "v1.2.3", got.AppVersion)
		assert.Equal(t, JobPhaseSucceeded, got.Phase)
		require.NotNil(t, got.ScheduledAt)
		assert.Equal(t, created.Time, *got.ScheduledAt)
		require.NotNil(t, got.StartedAt)
		assert.Equal(t, started.Time, *got.StartedAt)
		require.NotNil(t, got.CompletedAt)
		assert.Equal(t, completed.Time, *got.CompletedAt)
		assert.Empty(t, got.ExitReason)
	})

	t.Run("failed surfaces condition reason+message", func(t *testing.T) {
		failedAt := metav1.Time{Time: time.Date(2026, 5, 1, 12, 5, 0, 0, time.UTC)}
		job := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "broken-job", UID: types.UID("uid-2"), CreationTimestamp: created},
			Status: batchv1.JobStatus{
				StartTime: &started,
				Conditions: []batchv1.JobCondition{{
					Type:               batchv1.JobFailed,
					Status:             corev1.ConditionTrue,
					Reason:             "BackoffLimitExceeded",
					Message:            "Job has reached the specified backoff limit",
					LastTransitionTime: failedAt,
				}},
			},
		}
		got := AppStatusJobExecutionFromK8s(job)
		assert.Equal(t, JobPhaseFailed, got.Phase)
		assert.Equal(t, "BackoffLimitExceeded: Job has reached the specified backoff limit", got.ExitReason)
		// CompletionTime is unset for failed Jobs; we fall back to the failure transition time.
		require.NotNil(t, got.CompletedAt)
		assert.Equal(t, failedAt.Time, *got.CompletedAt)
	})

	t.Run("queued has only ScheduledAt", func(t *testing.T) {
		job := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "pending-job", UID: types.UID("uid-3"), CreationTimestamp: created},
		}
		got := AppStatusJobExecutionFromK8s(job)
		assert.Equal(t, JobPhaseQueued, got.Phase)
		require.NotNil(t, got.ScheduledAt)
		assert.Nil(t, got.StartedAt)
		assert.Nil(t, got.CompletedAt)
	})
}

func TestAppStatusJobSummaryFromK8s(t *testing.T) {
	now := metav1.Now()
	jobs := []batchv1.Job{
		// queued
		makeJob("a", nil),
		// running
		makeJob("b", func(j *batchv1.Job) { j.Status.Active = 1 }),
		// succeeded
		makeJob("c", func(j *batchv1.Job) {
			j.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		}),
		makeJob("d", func(j *batchv1.Job) {
			j.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		}),
		// failed
		makeJob("e", func(j *batchv1.Job) {
			j.Status.StartTime = &now
			j.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
		}),
	}
	summary := AppStatusJobSummaryFromK8s(jobs)
	assert.Equal(t, 5, summary.Created)
	assert.Equal(t, 2, summary.InProgress) // queued + running
	assert.Equal(t, 2, summary.Succeeded)
	assert.Equal(t, 1, summary.Failed)
}

func TestAppStatusJobSummaryFromK8s_Empty(t *testing.T) {
	summary := AppStatusJobSummaryFromK8s(nil)
	assert.Equal(t, AppStatusJobSummary{}, summary)
}
