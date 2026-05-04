package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

// NewLogStreamer returns an ECS-aware log streamer that translates the high-level
// Task / Deployment / Job filters into a list of CloudWatch log stream names, then
// delegates to the generic cloudwatch streamer.
func NewLogStreamer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.LogStreamer, error) {
	cwOuts, err := outputs.Retrieve[cloudwatch.Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	cwOuts.InitializeCreds(source, appDetails.Workspace)

	ecsOuts, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	ecsOuts.InitializeCreds(source, appDetails.Workspace)

	return LogStreamer{
		Inner: cloudwatch.LogStreamer{
			OsWriters: osWriters,
			Details:   appDetails,
			Infra:     cwOuts,
		},
		EcsInfra: ecsOuts,
	}, nil
}

type LogStreamer struct {
	Inner    cloudwatch.LogStreamer
	EcsInfra Outputs
}

func (l LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	streamNames, err := l.resolveStreamNames(ctx, options)
	if err != nil {
		return err
	}
	if streamNames != nil {
		options.LogStreamNames = streamNames
	}
	return l.Inner.Stream(ctx, options)
}

// resolveStreamNames translates the Task / Deployment / Job filters into the
// concrete list of CloudWatch log streams to scope on. Returns nil when no
// ECS-specific filter is set, signaling the cloudwatch streamer to stream the
// full log group as before.
func (l LogStreamer) resolveStreamNames(ctx context.Context, options app.LogStreamOptions) ([]string, error) {
	wantedTaskIds := map[string]struct{}{}
	if options.Task != "" {
		wantedTaskIds[options.Task] = struct{}{}
	}
	// ECS Job filter ≡ Task: it identifies a single one-off task ID.
	if options.Job != "" {
		wantedTaskIds[options.Job] = struct{}{}
	}
	if options.Deployment != "" {
		tasks, err := GetDeploymentTasks(ctx, l.EcsInfra, options.Deployment)
		if err != nil {
			return nil, fmt.Errorf("error resolving deployment %q to task IDs: %w", options.Deployment, err)
		}
		for _, t := range tasks {
			id := parseTaskId(t.TaskArn)
			if id != "" {
				wantedTaskIds[id] = struct{}{}
			}
		}
	}
	if len(wantedTaskIds) == 0 {
		return nil, nil
	}

	return l.findStreamsForTaskIds(ctx, wantedTaskIds)
}

// findStreamsForTaskIds enumerates the log streams in each log group covered by
// the workspace and returns the names whose final segment matches one of the
// wanted task IDs. ECS awslogs streams follow the form
// "<awslogs-stream-prefix>/<containerName>/<taskId>"; CloudWatch APIs do not
// support suffix matching, so we list and filter client-side.
func (l LogStreamer) findStreamsForTaskIds(ctx context.Context, wantedTaskIds map[string]struct{}) ([]string, error) {
	logGroupNames, err := cloudwatch.ExpandLogGroups(ctx, l.Inner.Infra)
	if err != nil {
		return nil, err
	}

	cwlClient := cloudwatchlogs.NewFromConfig(nsaws.NewConfig(l.Inner.Infra.LogReader, l.Inner.Infra.Region))
	var matched []string
	for _, lgName := range logGroupNames {
		var nextToken *string
		for {
			out, err := cwlClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
				LogGroupName: aws.String(lgName),
				NextToken:    nextToken,
				OrderBy:      cwltypes.OrderByLastEventTime,
				Descending:   aws.Bool(true),
			})
			if err != nil {
				return nil, fmt.Errorf("error listing log streams in %q: %w", lgName, err)
			}
			for _, s := range out.LogStreams {
				name := aws.ToString(s.LogStreamName)
				idx := strings.LastIndex(name, "/")
				if idx < 0 {
					continue
				}
				if _, ok := wantedTaskIds[name[idx+1:]]; ok {
					matched = append(matched, name)
				}
			}
			if out.NextToken == nil {
				break
			}
			nextToken = out.NextToken
		}
	}
	return matched, nil
}
