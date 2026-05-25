package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nullstone-io/deployment-sdk/display"
)

type LogStreamer interface {
	Stream(ctx context.Context, options LogStreamOptions) error
}

type LogStreamOptions struct {
	StartTime *time.Time
	EndTime   *time.Time

	// A pattern to filter logs.
	// If nil, no filter is applied to log messages
	// This may not be supported by all log providers.
	// For AWS Cloudwatch: https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html
	Pattern *string

	// A filter to apply when querying the log source
	// For Kubernetes, this is a label selector to further filter down the logs
	Selectors []string

	// A filter to apply when querying a Kubernetes log source for a single pod
	Pod string

	// Task scopes the log query to a single ECS task ID. Honored by the ECS log streamer.
	Task string

	// Deployment scopes the log query to all tasks belonging to the given ECS deployment ID.
	// Honored by the ECS log streamer; resolved to a list of task IDs at query time.
	Deployment string

	// Job scopes the log query to a job execution.
	// For Kubernetes, the request handler typically converts this into a label selector.
	// For ECS, this identifies a one-off task ID and is honored by the ECS log streamer
	// the same way Task is.
	Job string

	// Execution scopes the log query to a single Cloud Run job execution.
	// The value is the execution's short name (e.g. "my-job-slqpw"), which matches
	// the Cloud Logging label run.googleapis.com/execution_name. Honored by the
	// Cloud Logging streamer; ignored by other providers.
	Execution string

	// Revision scopes the log query to a single Cloud Run service revision.
	// The value is the revision name (e.g. "my-service-00001-abc"), which matches
	// the Cloud Logging label run.googleapis.com/revision_name. Honored by the
	// Cloud Logging streamer; ignored by other providers.
	Revision string

	// LogStreamNames is the resolved exact list of CloudWatch log streams to filter on.
	// Populated by an upstream streamer (e.g. the ECS shim) that translates Task/
	// Deployment/Job into stream names before delegating to the cloudwatch streamer.
	// CloudWatch's FilterLogEvents accepts at most 100 names per request; the cloudwatch
	// streamer chunks the list across goroutines as needed.
	LogStreamNames []string

	// WatchInterval dictates how often the log streamer will query AWS for new events
	// If left unspecified or 0, will use default watch interval of 1s
	// If a negative value is specified, watching will disable, the log streamer will terminate as soon as logs are emitted
	WatchInterval time.Duration

	Emitter LogEmitter

	// CancelFlushTimeout provides a way to configure how long to wait when flushing logs after a cancellation
	// This occurs when the user cancels or when a runner is done
	// This is currently supported for Kubernetes only
	// Specify 0 to skip flushing logs
	CancelFlushTimeout *time.Duration
	// StopFlushTimeout provides a way to configure how long to wait when flushing logs after a stop
	// This occurs when a pod stops
	// This is currently supported for Kubernetes only
	// Specify 0 to skip flushing logs
	StopFlushTimeout *time.Duration

	DebugLogger *log.Logger
}

func (o LogStreamOptions) QueryTimeMessage() string {
	if o.StartTime != nil {
		if o.EndTime != nil {
			return fmt.Sprintf("Querying logs between %s and %s", display.FormatTimePtr(o.StartTime), display.FormatTimePtr(o.EndTime))
		}
		return fmt.Sprintf("Querying logs starting %s", display.FormatTimePtr(o.StartTime))
	} else if o.EndTime != nil {
		return fmt.Sprintf("Querying logs until %s", display.FormatTimePtr(o.EndTime))
	}
	return fmt.Sprintf("Querying all logs")
}

func (o LogStreamOptions) WatchMessage() string {
	wi := o.WatchInterval
	if wi < 0 {
		return "Not watching logs"
	}
	if wi == 0 {
		wi = time.Second
	}
	return fmt.Sprintf("Watching logs (poll interval = %s)", wi)
}
