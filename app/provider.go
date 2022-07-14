package app

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

type Provider struct {
	NewPusher             NewPusherFunc
	NewDeployer           NewDeployerFunc
	NewDeployStatusGetter NewDeployStatusGetterFunc
}

type NewPusherFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (Pusher, error)
type NewDeployerFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (Deployer, error)
type NewDeployStatusGetterFunc func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (DeployStatusGetter, error)

type Pusher interface {
	Push(ctx context.Context, source, version string) error
}

type Deployer interface {
	Deploy(ctx context.Context, version string) (string, error)
}

type DeployStatusGetter interface {
	GetDeployStatus(ctx context.Context, reference string) (RolloutStatus, error)
}
