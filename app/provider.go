package app

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

type Provider struct {
	CanDeployImmediate bool
	NewPusher          NewPusherFunc
	NewDeployer        NewDeployerFunc
	NewDeployWatcher   NewDeployWatcherFunc
	NewStatuser        NewStatuserFunc
	NewLogStreamer     NewLogStreamerFunc
}

type NewPusherFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Pusher, error)
type NewDeployerFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Deployer, error)
type NewDeployStatusGetterFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployStatusGetter, error)
type NewDeployWatcherFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployWatcher, error)
type NewStatuserFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Statuser, error)
type NewLogStreamerFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (LogStreamer, error)

type Pusher interface {
	Push(ctx context.Context, source, version string) error
	CalculateVersion(ctx context.Context, commitSha string) (string, error)
}

type Deployer interface {
	Deploy(ctx context.Context, meta DeployMetadata) (string, error)
}

type DeployStatusGetter interface {
	GetDeployStatus(ctx context.Context, reference string) (RolloutStatus, error)
}

type DeployWatcher interface {
	Watch(ctx context.Context, reference string) error
}

type Statuser interface {
	StatusOverview(ctx context.Context) (any, error)
	Status(ctx context.Context) (any, error)
}
