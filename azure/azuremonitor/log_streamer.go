package azuremonitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

var (
	DefaultWatchInterval = 1 * time.Second
	LogAnalyticsScopes   = []string{"https://api.loganalytics.io/.default"}
)

func NewLogStreamer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.LogStreamer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return LogStreamer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type LogStreamer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (s LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	if options.WatchInterval == time.Duration(0) {
		options.WatchInterval = DefaultWatchInterval
	}
	if options.Emitter == nil {
		options.Emitter = app.NewWriterLogEmitter(os.Stdout)
	}

	logger := log.New(s.OsWriters.Stderr(), "", 0)
	logger.Println(options.QueryTimeMessage())
	logger.Println(options.WatchMessage())

	client, err := azquery.NewLogsClient(&s.Infra.LogReader, nil)
	if err != nil {
		return fmt.Errorf("error creating Log Analytics client: %w", err)
	}

	fn := s.writeLatestEventsFunc(client, options)
	for {
		if err := fn(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("error querying logs: %w", err)
		}
		if options.WatchInterval < 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(options.WatchInterval):
		}
	}
}

func (s LogStreamer) writeLatestEventsFunc(client *azquery.LogsClient, options app.LogStreamOptions) func(ctx context.Context) error {
	var lastEventTime *time.Time
	if options.StartTime != nil {
		t := *options.StartTime
		lastEventTime = &t
	}

	return func(ctx context.Context) error {
		query := s.buildKQLQuery(lastEventTime, options.EndTime)
		timespan := buildTimespan(lastEventTime, options.EndTime)

		body := azquery.Body{
			Query:    &query,
			Timespan: timespan,
		}
		resp, err := client.QueryWorkspace(ctx, s.Infra.WorkspaceId, body, nil)
		if err != nil {
			return fmt.Errorf("error executing KQL query: %w", err)
		}

		if resp.Tables == nil || len(resp.Tables) == 0 {
			return nil
		}

		table := resp.Tables[0]
		timeCol := findColumnIndex(table.Columns, "TimeGenerated")
		msgCol := findColumnIndex(table.Columns, "Log_s")
		if msgCol < 0 {
			msgCol = findColumnIndex(table.Columns, "Message")
		}
		sourceCol := findColumnIndex(table.Columns, "ContainerAppName_s")
		if sourceCol < 0 {
			sourceCol = findColumnIndex(table.Columns, "Source")
		}

		for _, row := range table.Rows {
			var ts time.Time
			var msg, source string

			if timeCol >= 0 && timeCol < len(row) {
				if tsStr, ok := row[timeCol].(string); ok {
					if parsed, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
						ts = parsed
					} else if parsed, err := time.Parse("2006-01-02T15:04:05Z", tsStr); err == nil {
						ts = parsed
					}
				}
			}
			if msgCol >= 0 && msgCol < len(row) {
				if m, ok := row[msgCol].(string); ok {
					msg = m
				}
			}
			if sourceCol >= 0 && sourceCol < len(row) {
				if src, ok := row[sourceCol].(string); ok {
					source = src
				}
			}

			if !ts.IsZero() {
				lastEventTime = &ts
			}
			options.Emitter(app.LogMessage{
				SourceType: "azure-monitor",
				Source:     source,
				Message:    msg,
				Timestamp:  ts,
			})
		}
		return nil
	}
}

func (s LogStreamer) buildKQLQuery(startTime *time.Time, endTime *time.Time) string {
	// Use the log_filter output if available, otherwise default to ContainerAppConsoleLogs_CL
	baseQuery := s.Infra.LogFilter
	if baseQuery == "" {
		baseQuery = "ContainerAppConsoleLogs_CL"
	}

	query := baseQuery
	if startTime != nil {
		query += fmt.Sprintf(" | where TimeGenerated >= datetime(%s)", startTime.UTC().Format(time.RFC3339))
	}
	if endTime != nil {
		query += fmt.Sprintf(" | where TimeGenerated <= datetime(%s)", endTime.UTC().Format(time.RFC3339))
	}
	query += " | order by TimeGenerated asc"
	return query
}

func buildTimespan(startTime *time.Time, endTime *time.Time) *azquery.TimeInterval {
	if startTime == nil && endTime == nil {
		ts := azquery.TimeInterval("PT1H")
		return &ts
	}
	start := "PT0S"
	if startTime != nil {
		start = startTime.UTC().Format(time.RFC3339)
	}
	end := time.Now().UTC().Format(time.RFC3339)
	if endTime != nil {
		end = endTime.UTC().Format(time.RFC3339)
	}
	ts := azquery.TimeInterval(fmt.Sprintf("%s/%s", start, end))
	return &ts
}

func findColumnIndex(columns []*azquery.Column, name string) int {
	for i, col := range columns {
		if col.Name != nil && *col.Name == name {
			return i
		}
	}
	return -1
}
