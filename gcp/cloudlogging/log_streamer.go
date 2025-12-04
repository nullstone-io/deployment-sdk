package cloudlogging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gcplogging "cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	DefaultWatchInterval = 1 * time.Second
	LogScopes            = []string{
		"https://www.googleapis.com/auth/logging.read",
	}
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
	logger.Println("Querying using the filter:")
	logger.Printf("\t%s\n", s.Infra.LogFilter)

	tokenSource, err := s.Infra.LogReader.TokenSource(ctx, LogScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := logadmin.NewClient(ctx, s.Infra.ProjectId, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating log client: %w", err)
	}
	defer client.Close()

	fn := s.writeLatestEventsFunc(s.Infra.LogFilter, options)
	for {
		if err := fn(ctx, client); err != nil {
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

func (s LogStreamer) writeLatestEventsFunc(filter string, options app.LogStreamOptions) func(ctx context.Context, client *logadmin.Client) error {
	startTime := options.StartTime
	selector := options.Selector
	var lastEventTime *time.Time
	visitedSpans := map[string]bool{}

	return func(ctx context.Context, client *logadmin.Client) error {
		if lastEventTime != nil {
			startTime = lastEventTime
		}
		it := client.Entries(ctx, logadmin.Filter(buildFilter(filter, selector, startTime, options.EndTime)))
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			entry, err := it.Next()
			if errors.Is(err, iterator.Done) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("failed to fetch log entry: %w", err)
			}
			if _, ok := visitedSpans[entry.SpanID]; ok {
				continue
			}
			lastEventTime = &entry.Timestamp
			visitedSpans[entry.SpanID] = true
			options.Emitter(LogMessageFromFilteredLogEvent(*entry))
		}
	}
}

func buildFilter(initialFilter string, selector *string, startTime *time.Time, endTime *time.Time) string {
	filters := []string{fmt.Sprintf("(%s)", initialFilter)}
	if selector != nil {
		filters = append(filters, *selector)
	}
	if startTime != nil {
		filters = append(filters, fmt.Sprintf("timestamp >= %q", startTime.Format(time.RFC3339)))
	}
	if endTime != nil {
		filters = append(filters, fmt.Sprintf("timestamp <= %q", endTime.Format(time.RFC3339)))
	}
	return strings.Join(filters, " AND ")
}

func LogMessageFromFilteredLogEvent(entry gcplogging.Entry) app.LogMessage {
	var msg string
	switch p := entry.Payload.(type) {
	case string:
		msg = p
	default:
		raw, _ := json.Marshal(entry.Payload)
		msg = string(raw)
	}
	return app.LogMessage{
		SourceType: "cloud-logging",
		Source:     entry.LogName,
		Stream:     "",
		Message:    msg,
		Timestamp:  entry.Timestamp,
	}
}
