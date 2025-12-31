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
	Selector *string

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
