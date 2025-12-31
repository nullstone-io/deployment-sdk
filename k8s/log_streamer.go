package k8s

import (
	"context"
	"os"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"k8s.io/client-go/rest"
)

type NewConfiger func(ctx context.Context) (*rest.Config, error)

type LogStreamer struct {
	OsWriters    logging.OsWriters
	Details      app.Details
	AppNamespace string
	AppName      string
	NewConfigFn  NewConfiger
}

func (l LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	if options.Emitter == nil {
		options.Emitter = app.NewWriterLogEmitter(os.Stdout)
	}

	streamer := NewWorkloadLogStreamer(l.NewConfigFn, options, l.AppNamespace, l.AppName)
	streamer.IsDebugEnabled = options.IsDebugEnabled
	if err := streamer.Stream(ctx); err != nil {
		return err
	}

	return nil
}
