package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"golang.org/x/sync/errgroup"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
	"os"
	"strings"
	"time"
)

var (
	DefaultWatchInterval = 1 * time.Second
)

func NewLogStreamer(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.LogStreamer, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

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

func (l LogStreamer) Stream(ctx context.Context, options app.LogStreamOptions) error {
	if options.WatchInterval == time.Duration(0) {
		options.WatchInterval = DefaultWatchInterval
	}
	if options.Emitter == nil {
		options.Emitter = app.NewWriterLogEmitter(os.Stdout)
	}

	logger := log.New(l.OsWriters.Stderr(), "", 0)
	logger.Println(options.QueryTimeMessage())
	logger.Println(options.WatchMessage())

	logGroupNames, err := ExpandLogGroups(context.Background(), l.Infra)
	if err != nil {
		return err
	}
	logger.Println("Querying the following log groups:")
	logger.Printf("\t%s\n", strings.Join(logGroupNames, "\n\t"))
	logger.Println()

	g, ctx := errgroup.WithContext(ctx)
	for _, logGroupName := range logGroupNames {
		g.Go(l.streamLogGroup(ctx, logGroupName, options))
	}
	return g.Wait()
}

func (l LogStreamer) streamLogGroup(ctx context.Context, logGroupName string, options app.LogStreamOptions) func() error {
	return func() error {
		fn := writeLatestEvents(l.Infra, logGroupName, options)
		for {
			if err := fn(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("error querying logs: %w", err)
			}
			if options.WatchInterval < 0 {
				// A negative watch interval indicates
				return nil
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(options.WatchInterval):
			}
		}
	}
}
