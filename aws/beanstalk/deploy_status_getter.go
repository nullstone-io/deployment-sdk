package beanstalk

import (
	"context"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

func NewDeployStatusGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return DeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type DeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	env, err := GetEnvironmentStatus(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	rolloutStatus := d.mapRolloutStatus(env)
	// TODO: Is there additional information to log?
	// TODO: Check env.Health for LB health checks?
	return rolloutStatus, nil
}

func (d DeployStatusGetter) mapRolloutStatus(env *ebtypes.EnvironmentDescription) app.RolloutStatus {
	switch env.Status {
	case "Launching":
		return app.RolloutStatusInProgress
	case "Updating":
		return app.RolloutStatusInProgress
	case "Ready":
		return app.RolloutStatusComplete
	case "Terminating":
		return app.RolloutStatusFailed
	case "Terminated":
		return app.RolloutStatusFailed
	default:
		return app.RolloutStatusUnknown
	}
}
