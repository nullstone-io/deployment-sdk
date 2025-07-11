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

type NewPusherFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Pusher, error)
type NewDeployerFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Deployer, error)
type NewDeployStatusGetterFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployStatusGetter, error)
type NewDeployWatcherFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployWatcher, error)
type NewStatuserFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Statuser, error)
type NewLogStreamerFunc func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (LogStreamer, error)

type Pusher interface {
	Push(ctx context.Context, source, version string) error
	ListArtifactVersions(ctx context.Context) ([]string, error)
}

type Deployer interface {
	Deploy(ctx context.Context, meta DeployMetadata) (string, error)
}

type DeployStatusGetter interface {
	GetDeployStatus(ctx context.Context, reference string) (RolloutStatus, error)
	Close()
}

type DeployWatcher interface {
	Watch(ctx context.Context, reference string, isFirstDeploy bool) error
}

type StatusOverviewResult interface {
	GetDeploymentVersions() []string
}

type Statuser interface {
	StatusOverview(ctx context.Context) (StatusOverviewResult, error)
	Status(ctx context.Context) (any, error)
}
