package cloudrun

import (
	"testing"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOutputsLocation(t *testing.T) {
	t.Run("uses explicit project_id/region", func(t *testing.T) {
		o := Outputs{ProjectId: "proj", Region: "us-central1"}
		assert.Equal(t, LocationInfo{ProjectId: "proj", Region: "us-central1"}, o.Location())
	})
	t.Run("parses from service_id", func(t *testing.T) {
		o := Outputs{ServiceId: "projects/proj/locations/us-east1/services/svc"}
		assert.Equal(t, LocationInfo{ProjectId: "proj", Region: "us-east1"}, o.Location())
	})
	t.Run("parses from job_id", func(t *testing.T) {
		o := Outputs{JobId: "projects/p2/locations/europe-west1/jobs/j"}
		assert.Equal(t, LocationInfo{ProjectId: "p2", Region: "europe-west1"}, o.Location())
	})
}

func TestShortName(t *testing.T) {
	assert.Equal(t, "svc-00012-abc", shortName("projects/p/locations/r/services/svc/revisions/svc-00012-abc"))
	assert.Equal(t, "bare", shortName("bare"))
	assert.Equal(t, "", shortName(""))
}

func TestDeriveRevisionLabel(t *testing.T) {
	assert.Equal(t, "rev 12", deriveRevisionLabel("fortuna-api-00012-abc", "fortuna-api"))
	assert.Equal(t, "rev 7", deriveRevisionLabel("fortuna-api-00007-xyz", "projects/p/locations/r/services/fortuna-api"))
	// Unparseable suffix falls back to the raw name.
	assert.Equal(t, "weird-name", deriveRevisionLabel("weird-name", "svc"))
}

func makeRevision(name string, opts func(*runpb.Revision)) *runpb.Revision {
	rev := &runpb.Revision{
		Name:       name,
		CreateTime: timestamppb.New(time.Now()),
		Conditions: []*runpb.Condition{
			{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
		},
	}
	if opts != nil {
		opts(rev)
	}
	return rev
}

func TestDeriveRevisionRole(t *testing.T) {
	s := Statuser{}
	svc := &runpb.Service{Name: "svc", LatestCreatedRevision: "svc-00005-aaa"}

	t.Run("latest serving", func(t *testing.T) {
		rev := makeRevision("svc-00005-aaa", nil)
		assert.Equal(t, RevisionRoleLatest, s.deriveRevisionRole(rev, svc, 100, nil))
	})
	t.Run("latest deployed but 0% traffic is stuck", func(t *testing.T) {
		rev := makeRevision("svc-00005-aaa", nil)
		assert.Equal(t, RevisionRoleStuck, s.deriveRevisionRole(rev, svc, 0, nil))
	})
	t.Run("prior serving", func(t *testing.T) {
		rev := makeRevision("svc-00004-bbb", nil)
		assert.Equal(t, RevisionRolePrior, s.deriveRevisionRole(rev, svc, 100, nil))
	})
	t.Run("tagged with no traffic", func(t *testing.T) {
		rev := makeRevision("svc-00003-ccc", nil)
		tag := &RevisionTag{Name: "preview", Url: "https://preview---svc.run.app"}
		assert.Equal(t, RevisionRoleTagged, s.deriveRevisionRole(rev, svc, 0, tag))
	})
	t.Run("idle", func(t *testing.T) {
		rev := makeRevision("svc-00002-ddd", nil)
		assert.Equal(t, RevisionRoleIdle, s.deriveRevisionRole(rev, svc, 0, nil))
	})
	t.Run("failed Ready condition wins over everything", func(t *testing.T) {
		rev := makeRevision("svc-00005-aaa", func(r *runpb.Revision) {
			r.Conditions = []*runpb.Condition{{Type: "Ready", State: runpb.Condition_CONDITION_FAILED}}
		})
		assert.Equal(t, RevisionRoleFailed, s.deriveRevisionRole(rev, svc, 100, nil))
	})
}

func TestMapRevision(t *testing.T) {
	s := Statuser{Infra: Outputs{MainContainerName: "main"}}
	svc := &runpb.Service{Name: "fortuna-api", LatestCreatedRevision: "fortuna-api-00012-abc"}
	rev := makeRevision("fortuna-api-00012-abc", func(r *runpb.Revision) {
		r.MaxInstanceRequestConcurrency = 80
		r.Scaling = &runpb.RevisionScaling{MinInstanceCount: 1, MaxInstanceCount: 100}
		r.ScalingStatus = &runpb.RevisionScalingStatus{DesiredMinInstanceCount: 3}
		r.Containers = []*runpb.Container{
			{
				Name:  "main",
				Image: "us-docker.pkg.dev/proj/repo/app:v1.2.3",
				Resources: &runpb.ResourceRequirements{
					Limits: map[string]string{"cpu": "1", "memory": "512Mi"},
				},
			},
		}
	})
	traffic := map[string]*runpb.TrafficTargetStatus{
		"fortuna-api-00012-abc": {Revision: "fortuna-api-00012-abc", Percent: 95},
	}

	out := s.mapRevision(rev, svc, traffic)
	assert.Equal(t, "fortuna-api-00012-abc", out.Name)
	assert.Equal(t, "rev 12", out.Label)
	assert.Equal(t, "v1.2.3", out.AppVersion)
	assert.Equal(t, int32(95), out.TrafficPercent)
	assert.Equal(t, int32(80), out.Concurrency)
	assert.Equal(t, int32(1), out.MinInstances)
	assert.Equal(t, int32(100), out.MaxInstances)
	assert.Equal(t, int32(3), out.InstanceCount)
	assert.Equal(t, "1", out.Cpu)
	assert.Equal(t, "512Mi", out.Memory)
	assert.Equal(t, RevisionRoleLatest, out.Role)
	assert.Nil(t, out.Failure)
	assert.NotNil(t, out.Instances) // never nil — serializes as []
}

func TestExecutionPhase(t *testing.T) {
	now := timestamppb.New(time.Now())
	cases := []struct {
		name string
		exec *runpb.Execution
		want JobExecutionPhase
	}{
		{
			name: "queued",
			exec: &runpb.Execution{},
			want: JobExecutionPhaseQueued,
		},
		{
			name: "running via start time",
			exec: &runpb.Execution{StartTime: now},
			want: JobExecutionPhaseRunning,
		},
		{
			name: "succeeded via completed condition",
			exec: &runpb.Execution{
				CompletionTime: now,
				TaskCount:      42, SucceededCount: 42,
				Conditions: []*runpb.Condition{{Type: "Completed", State: runpb.Condition_CONDITION_SUCCEEDED}},
			},
			want: JobExecutionPhaseSucceeded,
		},
		{
			name: "failed via completed condition",
			exec: &runpb.Execution{
				CompletionTime: now, TaskCount: 42, SucceededCount: 38, FailedCount: 4,
				Conditions: []*runpb.Condition{{Type: "Completed", State: runpb.Condition_CONDITION_FAILED}},
			},
			want: JobExecutionPhaseFailed,
		},
		{
			name: "failed via counts when no condition",
			exec: &runpb.Execution{CompletionTime: now, TaskCount: 42, SucceededCount: 38, FailedCount: 4},
			want: JobExecutionPhaseFailed,
		},
		{
			name: "succeeded via counts when no condition",
			exec: &runpb.Execution{CompletionTime: now, TaskCount: 42, SucceededCount: 42},
			want: JobExecutionPhaseSucceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, executionPhase(tc.exec))
		})
	}
}

func TestTaskState(t *testing.T) {
	now := timestamppb.New(time.Now())
	t.Run("queued", func(t *testing.T) {
		assert.Equal(t, TaskStateQueued, taskState(&runpb.Task{}))
	})
	t.Run("running", func(t *testing.T) {
		assert.Equal(t, TaskStateRunning, taskState(&runpb.Task{StartTime: now}))
	})
	t.Run("retrying", func(t *testing.T) {
		assert.Equal(t, TaskStateRetrying, taskState(&runpb.Task{StartTime: now, Retried: 1}))
	})
	t.Run("succeeded", func(t *testing.T) {
		task := &runpb.Task{CompletionTime: now, LastAttemptResult: &runpb.TaskAttemptResult{ExitCode: 0}}
		assert.Equal(t, TaskStateSucceeded, taskState(task))
	})
	t.Run("failed via nonzero exit", func(t *testing.T) {
		task := &runpb.Task{CompletionTime: now, LastAttemptResult: &runpb.TaskAttemptResult{ExitCode: 137}}
		assert.Equal(t, TaskStateFailed, taskState(task))
	})
}

func TestDeriveExecutionFailure(t *testing.T) {
	t.Run("nil when not failed", func(t *testing.T) {
		je := JobExecution{Phase: JobExecutionPhaseSucceeded}
		assert.Nil(t, deriveExecutionFailure(je, taskDiag{}))
	})
	t.Run("OOM when exit 137 detected", func(t *testing.T) {
		je := JobExecution{Phase: JobExecutionPhaseFailed, TaskCount: 42, FailedCount: 4}
		code := int32(137)
		diag := taskDiag{failedIndexes: []int32{17, 19, 28, 33}, oom: true, exitCode: &code}
		f := deriveExecutionFailure(je, diag)
		assert.NotNil(t, f)
		assert.Equal(t, FailureOOMKilled, f.Code)
		assert.Contains(t, f.Title, "OOMKilled")
		assert.Contains(t, f.Title, "4 tasks")
		require.NotNil(t, f.ExitCode)
		assert.Equal(t, int32(137), *f.ExitCode)
	})
	t.Run("generic failure otherwise", func(t *testing.T) {
		je := JobExecution{Phase: JobExecutionPhaseFailed, TaskCount: 10, FailedCount: 1}
		code := int32(2)
		diag := taskDiag{failedIndexes: []int32{3}, exitCode: &code}
		f := deriveExecutionFailure(je, diag)
		assert.NotNil(t, f)
		assert.Equal(t, FailureJobTaskFailed, f.Code)
		assert.Contains(t, f.Title, "1 task")
		require.NotNil(t, f.ExitCode)
		assert.Equal(t, int32(2), *f.ExitCode)
	})
}

func TestMapExecution(t *testing.T) {
	s := Statuser{Infra: Outputs{JobName: "batch-reindex", MainContainerName: "main"}}
	now := timestamppb.New(time.Now())
	exec := &runpb.Execution{
		Name:           "projects/p/locations/r/jobs/batch-reindex/executions/batch-reindex-0042",
		CreateTime:     now,
		StartTime:      now,
		TaskCount:      42,
		Parallelism:    8,
		SucceededCount: 17,
		RunningCount:   8,
		RetriedCount:   2,
		Template: &runpb.TaskTemplate{
			Retries:    &runpb.TaskTemplate_MaxRetries{MaxRetries: 3},
			Containers: []*runpb.Container{{Name: "main", Image: "us-docker.pkg.dev/p/repo/app:abc123"}},
		},
	}

	je := s.mapExecution(exec)
	assert.Equal(t, "batch-reindex", je.Name)
	assert.Equal(t, "batch-reindex-0042", je.ExecutionId)
	assert.Equal(t, "abc123", je.AppVersion)
	assert.Equal(t, int32(42), je.TaskCount)
	assert.Equal(t, int32(8), je.Parallelism)
	assert.Equal(t, int32(17), je.CompletedCount)
	assert.Equal(t, int32(8), je.RunningCount)
	assert.Equal(t, int32(2), je.RetryCount)
	assert.Equal(t, int32(3), je.MaxRetries)
	assert.Equal(t, JobExecutionPhaseRunning, je.Phase)
	assert.NotNil(t, je.Tasks) // never nil
}
