package cdn

import (
	"context"
	"fmt"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
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

	invalidation, err := GetInvalidation(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	} else if invalidation == nil {
		return app.RolloutStatusUnknown, fmt.Errorf("could not find invalidation")
	}
	return d.mapRolloutStatus(invalidation), nil
}

func (d DeployStatusGetter) mapRolloutStatus(invalidation *cftypes.Invalidation) app.RolloutStatus {
	stdout, stderr := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if invalidation == nil || invalidation.Status == nil {
		fmt.Fprintln(stderr, "Missing invalidation status")
		return app.RolloutStatusUnknown
	}
	switch *invalidation.Status {
	default:
		fmt.Fprintf(stderr, "Unknown invalidation status: %s\n", *invalidation.Status)
		return app.RolloutStatusUnknown
	case "InProgress":
		fmt.Fprintln(stdout, "Invalidating...")
		return app.RolloutStatusInProgress
	case "Completed":
		fmt.Fprintln(stdout, "Invalidation completed.")
		return app.RolloutStatusComplete
	}
}
