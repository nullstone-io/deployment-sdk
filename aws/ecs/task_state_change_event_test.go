package ecs

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// stoppedTaskEvent mirrors the shape EventBridge writes for a STOPPED task (ECS docs example, trimmed
// to one container plus the noise fields the parser must ignore: attachments, attributes, overrides).
const stoppedTaskEvent = `{
  "version": "0",
  "id": "105f6bb1-4da6-c630-4965-35383018cbca",
  "detail-type": "ECS Task State Change",
  "source": "aws.ecs",
  "account": "123456789012",
  "time": "2026-10-13T07:12:40Z",
  "region": "us-east-1",
  "resources": ["arn:aws:ecs:us-east-1:123456789012:task/example-cluster/a1173316d40a45dea9"],
  "detail": {
    "attachments": [{"id": "fe3a9a46", "type": "eni", "status": "DELETED", "details": [{"name": "subnetId", "value": "subnet-0d0e"}]}],
    "attributes": [{"name": "ecs.cpu-architecture", "value": "x86_64"}],
    "availabilityZone": "us-east-1b",
    "capacityProviderName": "FARGATE",
    "clusterArn": "arn:aws:ecs:us-east-1:123456789012:cluster/example-cluster",
    "connectivity": "CONNECTED",
    "connectivityAt": "2026-10-13T07:00:17.19Z",
    "containers": [
      {
        "containerArn": "arn:aws:ecs:us-east-1:123456789012:container/example-cluster/a1173316d40a45dea9/a0a99b87",
        "exitCode": %d,
        "lastStatus": "STOPPED",
        "name": "main",
        "image": "123456789012.dkr.ecr.us-east-1.amazonaws.com/sync-usage-stats:abc123",
        "imageDigest": "sha256:c15da6c91de8d2f436196f3a768483ad32c258ed4e1beb3d367a27ed67253e66",
        "runtimeId": "a1173316d40a45dea9-0265927825",
        "taskArn": "arn:aws:ecs:us-east-1:123456789012:task/example-cluster/a1173316d40a45dea9",
        "networkInterfaces": [{"attachmentId": "fe3a9a46", "privateIpv4Address": "10.0.1.163"}],
        "cpu": "0"
      }
    ],
    "cpu": "256",
    "createdAt": "2026-10-13T07:00:13.877Z",
    "desiredStatus": "STOPPED",
    "enableExecuteCommand": true,
    "ephemeralStorage": {"sizeInGiB": 20},
    "executionStoppedAt": "2026-10-13T07:12:38.1Z",
    "group": "family:sync-usage-stats-abcde",
    "launchType": "FARGATE",
    "lastStatus": "STOPPED",
    "memory": "512",
    "overrides": {
      "containerOverrides": [
        {"name": "main", "environment": [{"name": "NULLSTONE_TRIGGER", "value": "cron"}, {"name": "SECRET_OVERRIDE", "value": "hunter2"}]}
      ]
    },
    "platformVersion": "1.4.0",
    "pullStartedAt": "2026-10-13T07:00:24.162Z",
    "pullStoppedAt": "2026-10-13T07:00:33.493Z",
    "startedAt": "2026-10-13T07:00:34.325Z",
    "startedBy": "events-rule/sync-usage-stats-cron",
    "stopCode": "EssentialContainerExited",
    "stoppedAt": "2026-10-13T07:12:40.5Z",
    "stoppedReason": "Essential container in task exited",
    "stoppingAt": "2026-10-13T07:12:38.2Z",
    "taskArn": "arn:aws:ecs:us-east-1:123456789012:task/example-cluster/a1173316d40a45dea9",
    "taskDefinitionArn": "arn:aws:ecs:us-east-1:123456789012:task-definition/sync-usage-stats-abcde:5",
    "updatedAt": "2026-10-13T07:12:40.5Z",
	"version": 6
  }
}`

func TestParseTaskStateChangeEvent(t *testing.T) {
	event, err := parseTaskStateChangeEvent(sprintfEvent(stoppedTaskEvent, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := event.Detail
	if d.TaskArn != "arn:aws:ecs:us-east-1:123456789012:task/example-cluster/a1173316d40a45dea9" {
		t.Errorf("taskArn = %q", d.TaskArn)
	}
	if d.Group != "family:sync-usage-stats-abcde" {
		t.Errorf("group = %q", d.Group)
	}
	if d.Version != 6 {
		t.Errorf("version = %d, want 6", d.Version)
	}
	if d.LastStatus != "STOPPED" || d.StopCode != "EssentialContainerExited" {
		t.Errorf("lastStatus/stopCode = %q/%q", d.LastStatus, d.StopCode)
	}
	wantStopped := time.Date(2026, 10, 13, 7, 12, 40, 500_000_000, time.UTC)
	if d.StoppedAt == nil || !d.StoppedAt.Equal(wantStopped) {
		t.Errorf("stoppedAt = %v, want %v", d.StoppedAt, wantStopped)
	}
	if d.StartedAt == nil || d.CreatedAt == nil || d.PullStartedAt == nil {
		t.Errorf("expected startedAt, createdAt, pullStartedAt to be set")
	}
	if len(d.Containers) != 1 || d.Containers[0].Name != "main" || d.Containers[0].ExitCode == nil || *d.Containers[0].ExitCode != 0 {
		t.Errorf("containers = %+v", d.Containers)
	}
	if !event.Time.Equal(time.Date(2026, 10, 13, 7, 12, 40, 0, time.UTC)) {
		t.Errorf("time = %v", event.Time)
	}
}

func TestParseTaskStateChangeEvent_Rejects(t *testing.T) {
	for name, message := range map[string]string{
		"not json":      "ecs agent restarted",
		"no detail":     `{"version": "0", "source": "aws.ecs"}`,
		"empty taskArn": `{"detail": {"taskArn": ""}}`,
	} {
		if _, err := parseTaskStateChangeEvent(message); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// The history record must classify exactly like a live DescribeTasks result would.
func TestTaskStateChangeDetail_ToEcsTask_Phases(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		mutate   func(*taskStateChangeDetail)
		want     EcsJobExecutionPhase
	}{
		{name: "exit 0 is Succeeded", exitCode: 0, want: EcsJobPhaseSucceeded},
		{name: "exit 1 is Failed", exitCode: 1, want: EcsJobPhaseFailed},
		{name: "exit 137 (OOM) is Failed", exitCode: 137, want: EcsJobPhaseFailed},
		{
			name: "TaskFailedToStart is Failed even without exit codes", exitCode: 0,
			mutate: func(d *taskStateChangeDetail) {
				d.StopCode = string(ecstypes.TaskStopCodeTaskFailedToStart)
				d.Containers = nil
			},
			want: EcsJobPhaseFailed,
		},
		{
			name: "RUNNING is Running", exitCode: 0,
			mutate: func(d *taskStateChangeDetail) { d.LastStatus = "RUNNING"; d.StoppedAt = nil },
			want:   EcsJobPhaseRunning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parseTaskStateChangeEvent(sprintfEvent(stoppedTaskEvent, tt.exitCode))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.mutate != nil {
				tt.mutate(&event.Detail)
			}
			task := event.Detail.ToEcsTask()
			statusTask := StatusTaskFromEcsTask(task)
			exec := EcsJobExecutionFromStatusTask(statusTask, "abc123")
			if exec.Phase != tt.want {
				t.Errorf("phase = %s, want %s", exec.Phase, tt.want)
			}
			if exec.ExecutionId != "a1173316d40a45dea9" {
				t.Errorf("executionId = %q", exec.ExecutionId)
			}
			if exec.TaskDefinitionFamily != "sync-usage-stats-abcde" || exec.TaskDefinitionRevision != 5 {
				t.Errorf("task definition = %s:%d", exec.TaskDefinitionFamily, exec.TaskDefinitionRevision)
			}
			if exec.AppVersion != "abc123" || !exec.EnableExecuteCommand {
				t.Errorf("appVersion/enableExecuteCommand = %q/%v", exec.AppVersion, exec.EnableExecuteCommand)
			}
		})
	}
}

func TestTaskStateChangeDetail_ToEcsTask_CopiesTimestamps(t *testing.T) {
	event, _ := parseTaskStateChangeEvent(sprintfEvent(stoppedTaskEvent, 0))
	task := event.Detail.ToEcsTask()
	if aws.ToString(task.LastStatus) != "STOPPED" || task.StopCode != ecstypes.TaskStopCodeEssentialContainerExited {
		t.Errorf("status = %s / %s", aws.ToString(task.LastStatus), task.StopCode)
	}
	for name, got := range map[string]*time.Time{
		"createdAt": task.CreatedAt, "startedAt": task.StartedAt, "stoppedAt": task.StoppedAt,
		"stoppingAt": task.StoppingAt, "executionStoppedAt": task.ExecutionStoppedAt,
		"pullStartedAt": task.PullStartedAt, "pullStoppedAt": task.PullStoppedAt, "connectivityAt": task.ConnectivityAt,
	} {
		if got == nil {
			t.Errorf("%s not copied", name)
		}
	}
	if aws.ToString(task.StartedBy) != "events-rule/sync-usage-stats-cron" {
		t.Errorf("startedBy = %q", aws.ToString(task.StartedBy))
	}
}
