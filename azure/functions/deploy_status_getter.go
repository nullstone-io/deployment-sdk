package functions

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployStatusGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &DeployStatusGetter{
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

func (d *DeployStatusGetter) Close() {}

func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	stdout := d.OsWriters.Stdout()

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	// If the reference is a URL (from async zipdeploy Location header), poll it
	if len(reference) > 8 && reference[:8] == "https://" {
		return d.pollDeploymentStatus(ctx, reference)
	}

	// Otherwise, the deployment completed synchronously
	fmt.Fprintln(stdout, "Deployment completed.")
	return app.RolloutStatusComplete, nil
}

func (d *DeployStatusGetter) pollDeploymentStatus(ctx context.Context, statusUrl string) (app.RolloutStatus, error) {
	stdout := d.OsWriters.Stdout()

	armToken, err := d.Infra.Deployer.GetToken(ctx, policy.TokenRequestOptions{Scopes: ARMScopes})
	if err != nil {
		return app.RolloutStatusUnknown, fmt.Errorf("error obtaining ARM token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusUrl, nil)
	if err != nil {
		return app.RolloutStatusUnknown, fmt.Errorf("error creating status request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", armToken.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return app.RolloutStatusUnknown, fmt.Errorf("error polling deployment status: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Fprintln(stdout, "Deployment completed.")
		return app.RolloutStatusComplete, nil
	case http.StatusAccepted:
		fmt.Fprintln(stdout, "Deployment in progress...")
		return app.RolloutStatusInProgress, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Deployment status check returned HTTP %d: %s\n", resp.StatusCode, string(body))
		return app.RolloutStatusFailed, nil
	}
}
