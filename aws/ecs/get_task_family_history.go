package ecs

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

// historyTargetCount is how many distinct tasks are enough to stop widening the lookback window.
const historyTargetCount = 25

// historyLookbackWindows are read newest-first as non-overlapping chunks: [now-1d, now), [now-7d, now-1d), ...
// A minute-level cron fills the target from the first chunk; a nightly job widens out to the last one.
var historyLookbackWindows = []time.Duration{
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
	90 * 24 * time.Hour,
}

// GetTaskFamilyHistory reads the cluster's task state-change log for tasks in this app's family and
// returns the latest known state of each task, newest first. ECS forgets stopped tasks after about an
// hour; this is how executions stay visible past that.
// Returns nil when the cluster module does not record task events (no executions_log_group_name output).
func GetTaskFamilyHistory(ctx context.Context, infra Outputs, now time.Time) ([]ecstypes.Task, error) {
	logGroupName := infra.ExecutionsLogGroupName()
	if logGroupName == "" {
		return nil, nil
	}
	reader := taskHistoryReader{
		client:       cloudwatchlogs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region)),
		logGroupName: logGroupName,
		// Every RunTask without an explicit group (cron trigger, `nullstone run`) lands in "family:<family>".
		filterPattern: fmt.Sprintf(`{ $.detail.group = %q }`, "family:"+infra.TaskFamily()),
	}
	return reader.Read(ctx, now)
}

type logEventsFilterer interface {
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

type taskHistoryReader struct {
	client        logEventsFilterer
	logGroupName  string
	filterPattern string
}

func (r taskHistoryReader) Read(ctx context.Context, now time.Time) ([]ecstypes.Task, error) {
	latest := map[string]taskStateChangeDetail{}
	end := now
	for _, lookback := range historyLookbackWindows {
		start := now.Add(-lookback)
		if err := r.collect(ctx, start, end, latest); err != nil {
			return nil, err
		}
		if len(latest) >= historyTargetCount {
			break
		}
		end = start
	}

	tasks := make([]ecstypes.Task, 0, len(latest))
	for _, detail := range latest {
		tasks = append(tasks, detail.ToEcsTask())
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return aws.ToTime(tasks[i].CreatedAt).After(aws.ToTime(tasks[j].CreatedAt))
	})
	return tasks, nil
}

// collect folds every event in [start, end] into latest, keeping the highest task version per task ARN.
// Events are delivered at least once and can arrive out of order, so version decides, not position.
func (r taskHistoryReader) collect(ctx context.Context, start, end time.Time, latest map[string]taskStateChangeDetail) error {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(r.logGroupName),
		FilterPattern: aws.String(r.filterPattern),
		StartTime:     aws.Int64(start.UnixMilli()),
		EndTime:       aws.Int64(end.UnixMilli()),
	}
	for {
		out, err := r.client.FilterLogEvents(ctx, input)
		if err != nil {
			return fmt.Errorf("unable to read task execution history from log group %q: %w", r.logGroupName, err)
		}
		for _, event := range out.Events {
			parsed, err := parseTaskStateChangeEvent(aws.ToString(event.Message))
			if err != nil {
				// Not a task event (or a malformed one); it cannot describe an execution, so skip it.
				continue
			}
			if existing, ok := latest[parsed.Detail.TaskArn]; ok && existing.Version >= parsed.Detail.Version {
				continue
			}
			latest[parsed.Detail.TaskArn] = parsed.Detail
		}
		if out.NextToken == nil {
			return nil
		}
		input.NextToken = out.NextToken
	}
}
