package ecs

import (
	"testing"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestDeriveEcsJobPhase(t *testing.T) {
	int32p := func(v int32) *int32 { return &v }

	tests := []struct {
		name string
		task StatusTask
		want EcsJobExecutionPhase
	}{
		{
			name: "PROVISIONING is Queued",
			task: StatusTask{Status: "PROVISIONING"},
			want: EcsJobPhaseQueued,
		},
		{
			name: "PENDING is Queued",
			task: StatusTask{Status: "PENDING"},
			want: EcsJobPhaseQueued,
		},
		{
			name: "RUNNING is Running",
			task: StatusTask{Status: "RUNNING"},
			want: EcsJobPhaseRunning,
		},
		{
			name: "ACTIVATING is Running",
			task: StatusTask{Status: "ACTIVATING"},
			want: EcsJobPhaseRunning,
		},
		{
			name: "STOPPING is Running",
			task: StatusTask{Status: "STOPPING"},
			want: EcsJobPhaseRunning,
		},
		{
			name: "DEPROVISIONING is Running",
			task: StatusTask{Status: "DEPROVISIONING"},
			want: EcsJobPhaseRunning,
		},
		{
			name: "STOPPED with all-zero exit codes is Succeeded",
			task: StatusTask{Status: "STOPPED", Containers: []StatusTaskContainer{{ExitCode: int32p(0)}}},
			want: EcsJobPhaseSucceeded,
		},
		{
			name: "STOPPED with multi-container all-zero exits is Succeeded",
			task: StatusTask{Status: "STOPPED", Containers: []StatusTaskContainer{{ExitCode: int32p(0)}, {ExitCode: int32p(0)}}},
			want: EcsJobPhaseSucceeded,
		},
		{
			name: "STOPPED with non-zero exit is Failed",
			task: StatusTask{Status: "STOPPED", Containers: []StatusTaskContainer{{ExitCode: int32p(1)}}},
			want: EcsJobPhaseFailed,
		},
		{
			name: "STOPPED with mixed exits where one is non-zero is Failed",
			task: StatusTask{Status: "STOPPED", Containers: []StatusTaskContainer{{ExitCode: int32p(0)}, {ExitCode: int32p(137)}}},
			want: EcsJobPhaseFailed,
		},
		{
			name: "STOPPED with TaskFailedToStart is Failed regardless of containers",
			task: StatusTask{Status: "STOPPED", StopCode: ecstypes.TaskStopCodeTaskFailedToStart},
			want: EcsJobPhaseFailed,
		},
		{
			name: "STOPPED with no exit codes and UserInitiated is Succeeded",
			task: StatusTask{Status: "STOPPED", StopCode: ecstypes.TaskStopCodeUserInitiated},
			want: EcsJobPhaseSucceeded,
		},
		{
			name: "STOPPED with no exit codes and EssentialContainerExited is Succeeded",
			task: StatusTask{Status: "STOPPED", StopCode: ecstypes.TaskStopCodeEssentialContainerExited},
			want: EcsJobPhaseSucceeded,
		},
		{
			name: "STOPPED with no exit codes and SpotInterruption is Failed",
			task: StatusTask{Status: "STOPPED", StopCode: ecstypes.TaskStopCodeSpotInterruption},
			want: EcsJobPhaseFailed,
		},
		{
			name: "STOPPED with no exit codes and TerminationNotice is Failed",
			task: StatusTask{Status: "STOPPED", StopCode: ecstypes.TaskStopCodeTerminationNotice},
			want: EcsJobPhaseFailed,
		},
		{
			name: "STOPPED with no exit codes and empty StopCode is Failed (conservative)",
			task: StatusTask{Status: "STOPPED"},
			want: EcsJobPhaseFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveEcsJobPhase(tt.task); got != tt.want {
				t.Errorf("deriveEcsJobPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEcsJobExecutionFromStatusTask(t *testing.T) {
	int32p := func(v int32) *int32 { return &v }
	task := StatusTask{
		Id:                     "abc123",
		TaskArn:                "arn:aws:ecs:us-east-1:111:task/my-cluster/abc123",
		TaskDefinitionFamily:   "nightly-sync",
		TaskDefinitionRevision: 7,
		EnableExecuteCommand:   true,
		StartedAt:              nil,
		StoppedAt:              nil,
		StoppedReason:          "Essential container in task exited",
		StopCode:               ecstypes.TaskStopCodeEssentialContainerExited,
		Status:                 "STOPPED",
		Containers:             []StatusTaskContainer{{Name: "main", ExitCode: int32p(0)}},
	}

	got := EcsJobExecutionFromStatusTask(task, "v1.2.3")

	if got.ExecutionId != "abc123" {
		t.Errorf("ExecutionId = %q, want %q", got.ExecutionId, "abc123")
	}
	if got.AppVersion != "v1.2.3" {
		t.Errorf("AppVersion = %q, want %q", got.AppVersion, "v1.2.3")
	}
	if got.Phase != EcsJobPhaseSucceeded {
		t.Errorf("Phase = %v, want %v", got.Phase, EcsJobPhaseSucceeded)
	}
	if got.TaskDefinitionFamily != "nightly-sync" || got.TaskDefinitionRevision != 7 {
		t.Errorf("TaskDefinition family/revision = %q/%d, want nightly-sync/7", got.TaskDefinitionFamily, got.TaskDefinitionRevision)
	}
	if !got.EnableExecuteCommand {
		t.Errorf("EnableExecuteCommand = false, want true")
	}
}
