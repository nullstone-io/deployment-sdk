package functions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice/v2"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

var (
	// ARMScopes are the ARM management scopes for Azure Resource Manager.
	ARMScopes = []string{"https://management.azure.com/.default"}
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return Deployer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved Azure Functions outputs")
	fmt.Fprintf(stdout, "\tsubscription_id:   %s\n", d.Infra.SubscriptionId)
	fmt.Fprintf(stdout, "\tresource_group:    %s\n", d.Infra.ResourceGroup)
	fmt.Fprintf(stdout, "\tfunction_app_name: %s\n", d.Infra.FunctionAppName)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	// Get the publish profile to obtain the Kudu credentials
	webClient, err := armappservice.NewWebAppsClient(d.Infra.SubscriptionId, &d.Infra.Deployer, nil)
	if err != nil {
		return "", fmt.Errorf("error creating web apps client: %w", err)
	}

	// Use zipdeploy via the SCM site with the ARM bearer token
	armToken, err := d.Infra.Deployer.GetToken(ctx, policy.TokenRequestOptions{Scopes: ARMScopes})
	if err != nil {
		return "", fmt.Errorf("error obtaining ARM token: %w", err)
	}

	// Read the zip artifact from the local filesystem
	// The artifact is expected to have been pushed by a pusher (e.g., blob pusher) beforehand
	// For Azure Functions, we deploy using zipdeploy via the SCM endpoint
	zipPath := meta.Version
	if meta.PackageMode != "" {
		zipPath = meta.PackageMode
	}

	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		// If we can't read a local zip, try deploying via the ARM API with the version as a reference
		fmt.Fprintf(stdout, "No local zip file found at %q, deploying via ARM restart...\n", zipPath)
		return d.deployViaRestart(ctx, webClient, meta)
	}

	// Deploy via Kudu zipdeploy endpoint
	scmEndpoint := fmt.Sprintf("https://%s.scm.azurewebsites.net/api/zipdeploy?isAsync=true", d.Infra.FunctionAppName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scmEndpoint, bytes.NewReader(zipData))
	if err != nil {
		return "", fmt.Errorf("error creating zipdeploy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", armToken.Token))

	fmt.Fprintf(stdout, "Deploying zip to %s...\n", scmEndpoint)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error deploying zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("zipdeploy returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// The Location header contains the URL to poll for deployment status
	deploymentId := resp.Header.Get("Location")
	if deploymentId == "" {
		deploymentId = meta.Version
	}

	fmt.Fprintf(stdout, "Deployed app %q successfully\n", d.Details.App.Name)
	return deploymentId, nil
}

func (d Deployer) deployViaRestart(ctx context.Context, webClient *armappservice.WebAppsClient, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	// Sync function triggers and restart the function app
	fmt.Fprintln(stdout, "Syncing function triggers...")
	_, err := webClient.SyncFunctionTriggers(ctx, d.Infra.ResourceGroup, d.Infra.FunctionAppName, nil)
	if err != nil {
		return "", fmt.Errorf("error syncing function triggers: %w", err)
	}

	fmt.Fprintln(stdout, "Restarting function app...")
	_, err = webClient.Restart(ctx, d.Infra.ResourceGroup, d.Infra.FunctionAppName, nil)
	if err != nil {
		return "", fmt.Errorf("error restarting function app: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed app %q successfully\n", d.Details.App.Name)
	return meta.Version, nil
}
