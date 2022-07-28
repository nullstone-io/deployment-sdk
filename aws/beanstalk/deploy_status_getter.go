package beanstalk

import (
	"context"
	"fmt"
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
	} else if env == nil {
		return app.RolloutStatusInProgress, nil
	}
	rolloutStatus := d.mapRolloutStatus(env)
	// TODO: Is there additional information to log?
	return rolloutStatus, nil
}

func (d DeployStatusGetter) mapRolloutStatus(env *ebtypes.EnvironmentDescription) app.RolloutStatus {
	stdout := d.OsWriters.Stdout()
	switch env.Status {
	case ebtypes.EnvironmentStatusLaunching:
		fallthrough
	case ebtypes.EnvironmentStatusUpdating:
		fallthrough
	case ebtypes.EnvironmentStatusLinkingTo:
		fallthrough
	case ebtypes.EnvironmentStatusLinkingFrom:
		fmt.Fprintf(stdout, "Awaiting environment to launch (currently: %s)\n", env.Status)
		return app.RolloutStatusInProgress
	case ebtypes.EnvironmentStatusTerminating:
		return app.RolloutStatusFailed
	case ebtypes.EnvironmentStatusTerminated:
		return app.RolloutStatusFailed
	default:
		return app.RolloutStatusUnknown
	case ebtypes.EnvironmentStatusReady:
		// fall through to check health status
	}

	fmt.Fprintf(stdout,"Awaiting environment health to become healthy (currently: %s)\n", env.Health)
	switch env.Health {
	case ebtypes.EnvironmentHealthGreen:
		return app.RolloutStatusComplete
	case ebtypes.EnvironmentHealthYellow:
		return app.RolloutStatusInProgress
	case ebtypes.EnvironmentHealthRed:
		return app.RolloutStatusFailed
	case ebtypes.EnvironmentHealthGrey:
		return app.RolloutStatusFailed
	}
	return app.RolloutStatusUnknown
}
