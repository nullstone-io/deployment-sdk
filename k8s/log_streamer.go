package k8s

import (
	"context"
	"fmt"
	"os"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s/logs"
	"github.com/nullstone-io/deployment-sdk/logging"
)

type LogStreamer struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  logs.NewConfiger
}

func (l LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	if options.Emitter == nil {
		options.Emitter = app.NewWriterLogEmitter(os.Stdout)
	}
	selector := fmt.Sprintf("nullstone.io/app=%s", l.AppName)
	if options.Selector != nil && *options.Selector != "" {
		selector = *options.Selector
	}

	streamer := logs.WorkloadStreamer{
		Namespace:    l.AppNamespace,
		WorkloadName: l.AppName,
		NewConfigFn:  l.NewConfigFn,
		Options:      options,
		Selector:     selector,
	}
	return streamer.Stream(ctx)
}
