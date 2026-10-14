package ecs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func sprintfEvent(template string, exitCode int) string {
	return fmt.Sprintf(template, exitCode)
}

// taskEvent builds a minimal task state-change message for a given task, version and status.
func taskEvent(taskId string, version int64, lastStatus string, createdAt time.Time) string {
	return fmt.Sprintf(`{"time": %q, "detail": {"taskArn": "arn:aws:ecs:us-east-1:123:task/c/%s", "taskDefinitionArn": "arn:aws:ecs:us-east-1:123:task-definition/fam:1", "group": "family:fam", "lastStatus": %q, "version": %d, "createdAt": %q}}`,
		createdAt.Format(time.RFC3339Nano), taskId, lastStatus, version, createdAt.Format(time.RFC3339Nano))
}

type fakeLogEvents struct {
	// events keyed by log event timestamp (ms); pageSize splits each query's results into pages
	events   map[int64][]string
	pageSize int
	calls    []*cloudwatchlogs.FilterLogEventsInput
	err      error
}

func (f *fakeLogEvents) FilterLogEvents(_ context.Context, params *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	f.calls = append(f.calls, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  params.LogGroupName,
		FilterPattern: params.FilterPattern,
		StartTime:     params.StartTime,
		EndTime:       params.EndTime,
		NextToken:     params.NextToken,
	})
	if f.err != nil {
		return nil, f.err
	}
	matched := make([]cwltypes.FilteredLogEvent, 0)
	for ts, messages := range f.events {
		if ts < aws.ToInt64(params.StartTime) || ts > aws.ToInt64(params.EndTime) {
			continue
		}
		for _, m := range messages {
			matched = append(matched, cwltypes.FilteredLogEvent{Timestamp: aws.Int64(ts), Message: aws.String(m)})
		}
	}
	offset := 0
	if params.NextToken != nil {
		fmt.Sscanf(*params.NextToken, "page:%d", &offset)
	}
	end := len(matched)
	var next *string
	if f.pageSize > 0 && offset+f.pageSize < len(matched) {
		end = offset + f.pageSize
		next = aws.String(fmt.Sprintf("page:%d", end))
	}
	return &cloudwatchlogs.FilterLogEventsOutput{Events: matched[offset:end], NextToken: next}, nil
}

func TestTaskHistoryReader_KeepsHighestVersionPerTask(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	created := now.Add(-2 * time.Hour)
	fake := &fakeLogEvents{events: map[int64][]string{
		// out of order on purpose: STOPPED (v5) arrives before RUNNING (v3) and a duplicate STOPPED
		now.Add(-90 * time.Minute).UnixMilli():  {taskEvent("a", 5, "STOPPED", created)},
		now.Add(-100 * time.Minute).UnixMilli(): {taskEvent("a", 3, "RUNNING", created), taskEvent("a", 5, "STOPPED", created)},
		now.Add(-80 * time.Minute).UnixMilli():  {taskEvent("b", 2, "PENDING", created.Add(time.Minute)), "not an event"},
	}}
	reader := taskHistoryReader{client: fake, logGroupName: "cluster-executions", filterPattern: `{ $.detail.group = "family:fam" }`}

	tasks, err := reader.Read(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	// newest createdAt first
	if !strings.HasSuffix(aws.ToString(tasks[0].TaskArn), "/b") || !strings.HasSuffix(aws.ToString(tasks[1].TaskArn), "/a") {
		t.Errorf("order = %s, %s", aws.ToString(tasks[0].TaskArn), aws.ToString(tasks[1].TaskArn))
	}
	if aws.ToString(tasks[1].LastStatus) != "STOPPED" || tasks[1].Version != 5 {
		t.Errorf("task a = %s v%d, want STOPPED v5", aws.ToString(tasks[1].LastStatus), tasks[1].Version)
	}
	first := fake.calls[0]
	if aws.ToString(first.LogGroupName) != "cluster-executions" || aws.ToString(first.FilterPattern) != `{ $.detail.group = "family:fam" }` {
		t.Errorf("first call = %+v", first)
	}
}

func TestTaskHistoryReader_WidensWindowUntilTargetReached(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	fake := &fakeLogEvents{events: map[int64][]string{}}
	// one nightly run per day for the last 40 days
	for day := 0; day < 40; day++ {
		ts := now.Add(-time.Duration(day)*24*time.Hour - time.Hour)
		fake.events[ts.UnixMilli()] = []string{taskEvent(fmt.Sprintf("d%d", day), 6, "STOPPED", ts)}
	}
	reader := taskHistoryReader{client: fake, logGroupName: "g", filterPattern: "p"}

	tasks, err := reader.Read(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1d chunk → 1 task, 7d → 7, 30d → 30 ≥ target: stop before the 90d chunk
	if len(tasks) != 30 {
		t.Errorf("got %d tasks, want 30", len(tasks))
	}
	if len(fake.calls) != 3 {
		t.Fatalf("got %d calls, want 3 (1d, 7d, 30d)", len(fake.calls))
	}
	// chunks must not overlap: each query ends where the previous one started
	for i := 1; i < len(fake.calls); i++ {
		if aws.ToInt64(fake.calls[i].EndTime) != aws.ToInt64(fake.calls[i-1].StartTime) {
			t.Errorf("call %d end %d != call %d start %d", i, aws.ToInt64(fake.calls[i].EndTime), i-1, aws.ToInt64(fake.calls[i-1].StartTime))
		}
	}
	if aws.ToInt64(fake.calls[0].EndTime) != now.UnixMilli() || aws.ToInt64(fake.calls[0].StartTime) != now.Add(-24*time.Hour).UnixMilli() {
		t.Errorf("first window = [%d, %d]", aws.ToInt64(fake.calls[0].StartTime), aws.ToInt64(fake.calls[0].EndTime))
	}
}

func TestTaskHistoryReader_ReadsAllWindowsForSparseHistory(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-60 * 24 * time.Hour)
	fake := &fakeLogEvents{events: map[int64][]string{ts.UnixMilli(): {taskEvent("old", 6, "STOPPED", ts)}}}
	reader := taskHistoryReader{client: fake, logGroupName: "g", filterPattern: "p"}

	tasks, err := reader.Read(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || len(fake.calls) != len(historyLookbackWindows) {
		t.Errorf("got %d tasks over %d calls, want 1 task over %d calls", len(tasks), len(fake.calls), len(historyLookbackWindows))
	}
}

func TestTaskHistoryReader_FollowsPagination(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-time.Hour)
	messages := make([]string, 0)
	for i := 0; i < 7; i++ {
		messages = append(messages, taskEvent(fmt.Sprintf("t%d", i), 6, "STOPPED", ts))
	}
	fake := &fakeLogEvents{events: map[int64][]string{ts.UnixMilli(): messages}, pageSize: 3}
	reader := taskHistoryReader{client: fake, logGroupName: "g", filterPattern: "p"}

	tasks, err := reader.Read(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 7 {
		t.Errorf("got %d tasks, want 7", len(tasks))
	}
	paged := 0
	for _, c := range fake.calls {
		if c.NextToken != nil {
			paged++
		}
	}
	if paged != 2 {
		t.Errorf("got %d paged calls, want 2", paged)
	}
}

func TestTaskHistoryReader_PropagatesErrors(t *testing.T) {
	fake := &fakeLogEvents{err: errors.New("AccessDeniedException")}
	reader := taskHistoryReader{client: fake, logGroupName: "g", filterPattern: "p"}
	if _, err := reader.Read(context.Background(), time.Now()); err == nil || !strings.Contains(err.Error(), "AccessDeniedException") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestGetTaskFamilyHistory_NoLogGroupIsNoop(t *testing.T) {
	tasks, err := GetTaskFamilyHistory(context.Background(), Outputs{}, time.Now())
	if err != nil || tasks != nil {
		t.Errorf("got %v, %v; want nil, nil", tasks, err)
	}
}

func TestOutputs_ExecutionsLogGroupName(t *testing.T) {
	o := Outputs{Cluster: ClusterOutputs{ExecutionsLogGroupName: "cluster"}}
	if o.ExecutionsLogGroupName() != "cluster" {
		t.Errorf("cluster fallback = %q", o.ExecutionsLogGroupName())
	}
	o.ClusterNamespace.ExecutionsLogGroupName = "preview"
	if o.ExecutionsLogGroupName() != "preview" {
		t.Errorf("namespace precedence = %q", o.ExecutionsLogGroupName())
	}
}

func TestNewestExecutions(t *testing.T) {
	at := func(h int) *time.Time { t := time.Date(2026, 10, 14, h, 0, 0, 0, time.UTC); return &t }
	execs := []EcsJobExecution{
		{ExecutionId: "old", StartedAt: at(1), StoppedAt: at(2)},
		{ExecutionId: "running", StartedAt: at(5)},
		{ExecutionId: "new", StartedAt: at(3), StoppedAt: at(4)},
		{ExecutionId: "never-started"},
	}
	got := newestExecutions(execs, 3)
	ids := make([]string, 0)
	for _, e := range got {
		ids = append(ids, e.ExecutionId)
	}
	// Live executions (no StoppedAt) always survive the cap; stopped ones are kept newest-first.
	if want := "running,never-started,new"; strings.Join(ids, ",") != want {
		t.Errorf("order = %s, want %s", strings.Join(ids, ","), want)
	}
}
