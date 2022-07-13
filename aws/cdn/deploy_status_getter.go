package cdn

import (
	"context"
	"fmt"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
)

func NewDeployStatusGetter(logger *log.Logger, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return DeployStatusGetter{
		Logger:   logger,
		NsConfig: nsConfig,
		Details:  appDetails,
		Infra:    outs,
	}, nil
}

type DeployStatusGetter struct {
	Logger   *log.Logger
	NsConfig api.Config
	Details  app.Details
	Infra    Outputs
}

func (d DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	invalidation, err := GetInvalidation(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	} else if invalidation == nil {
		return app.RolloutStatusUnknown, fmt.Errorf("could not find invalidation")
	}
	return d.mapRolloutStatus(invalidation), nil
}

func (d DeployStatusGetter) mapRolloutStatus(invalidation *cftypes.Invalidation) app.RolloutStatus {
	if invalidation == nil || invalidation.Status == nil {
		d.Logger.Println("Missing invalidation status")
		return app.RolloutStatusUnknown
	}
	switch *invalidation.Status {
	default:
		d.Logger.Printf("Unknown invalidation status: %s\n", *invalidation.Status)
		return app.RolloutStatusUnknown
	case "InProgress":
		d.Logger.Println("Invalidating...")
		return app.RolloutStatusInProgress
	case "Completed":
		d.Logger.Println("Invalidation completed.")
		return app.RolloutStatusComplete
	}
}
