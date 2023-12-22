package app

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

type Provider struct {
	CanDeployImmediate bool
	NewPusher          NewPusherFunc
	NewDeployer        NewDeployerFunc
	NewDeployWatcher   NewDeployWatcherFunc
	NewStatuser        NewStatuserFunc
	NewLogStreamer     NewLogStreamerFunc
	NewMetricsGetter   NewMetricsGetterFunc
}

type NewPusherFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (Pusher, error)
type NewDeployerFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (Deployer, error)
type NewDeployStatusGetterFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (DeployStatusGetter, error)
type NewDeployWatcherFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (DeployWatcher, error)
type NewStatuserFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (Statuser, error)
type NewLogStreamerFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (LogStreamer, error)
type NewMetricsGetterFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (MetricsGetter, error)

type Pusher interface {
	Push(ctx context.Context, source, version string) error
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
