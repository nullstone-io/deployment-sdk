package acr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/mitchellh/colorstring"
	"github.com/moby/moby/client"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var (
	// ARMScopes are the ARM management scopes needed for the ACR token exchange.
	ARMScopes = []string{"https://management.azure.com/.default"}
)

type Outputs struct {
	RegistryUrl  string          `ns:"registry_url,optional"`
	ImageRepoUrl docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher  azure.Principal `ns:"image_pusher,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.ImagePusher.InitializeCreds(source, ws, types.AutomationPurposePush, "image_pusher")
}

func NewPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)
	return &Pusher{
		OsWriters: osWriters,
		Infra:     outs,
	}, nil
}

type Pusher struct {
	OsWriters logging.OsWriters
	Infra     Outputs
}

func (p Pusher) Print() {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved ACR outputs")
	fmt.Fprintf(stdout, "\tregistry_url:   %s\n", p.Infra.RegistryUrl)
	fmt.Fprintf(stdout, "\timage_repo_url: %s\n", p.Infra.ImageRepoUrl)
	fmt.Fprintf(stdout, "\timage_pusher:   %s/%s\n", p.Infra.ImagePusher.TenantId, p.Infra.ImagePusher.ClientId)
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	p.Print()

	sourceUrl := docker.ParseImageUrl(source)
	targetUrl := p.Infra.ImageRepoUrl
	targetUrl.Tag = version

	if err := p.validate(targetUrl); err != nil {
		return err
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Authenticating with ACR...")
	targetAuth, err := p.getAcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving ACR credentials: %w", err)
	}
	fmt.Fprintln(stdout, "Authenticated")

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintf(stdout, "Retagging source image %s => %s\n", sourceUrl, targetUrl)
	opts := client.ImageTagOptions{Source: sourceUrl.String(), Target: targetUrl.String()}
	if _, err := dockerCli.Client().ImageTag(ctx, opts); err != nil {
		return fmt.Errorf("error retagging image: %w", err)
	}

	fmt.Fprintln(stdout)
	colorstring.Fprintf(stdout, "[bold]Pushing docker image to %s\n", targetUrl)
	if err := docker.PushImage(ctx, dockerCli, targetUrl, targetAuth); err != nil {
		return fmt.Errorf("error pushing image: %w", err)
	}

	return nil
}

func (p Pusher) Pull(ctx context.Context, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	p.Print()

	sourceUrl := p.Infra.ImageRepoUrl
	sourceUrl.Tag = version
	if err := p.validate(sourceUrl); err != nil {
		return err
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Authenticating with ACR...")
	sourceAuth, err := p.getAcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving ACR credentials: %w", err)
	}
	fmt.Fprintln(stdout, "Authenticated")

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintln(stdout)
	colorstring.Fprintf(stdout, "[bold]Pulling docker image %s\n", sourceUrl)
	if err := docker.PullImage(ctx, dockerCli, sourceUrl, sourceAuth); err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	return nil
}

func (p Pusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	auth, err := p.getAcrLoginAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving ACR credentials: %w", err)
	}
	tags, err := docker.ListRemoteTags(ctx, p.Infra.ImageRepoUrl, auth)
	if err != nil {
		return nil, fmt.Errorf("error listing remote tags: %w", err)
	}
	return tags, nil
}

func (p Pusher) validate(imageUrl docker.ImageUrl) error {
	if imageUrl.String() == "" {
		return fmt.Errorf("cannot push if 'image_repo_url' module output is missing")
	}
	if imageUrl.Tag == "" {
		return fmt.Errorf("no version was specified, version is required to push image")
	}
	registry := imageUrl.Registry
	if !strings.HasSuffix(registry, ".azurecr.io") {
		return fmt.Errorf("this pusher only supports Azure Container Registry (*.azurecr.io), got %q", registry)
	}
	return nil
}

// getAcrLoginAuth performs the AAD->ACR OAuth2 token exchange chain and returns Docker auth config.
func (p Pusher) getAcrLoginAuth(ctx context.Context) (dockerregistry.AuthConfig, error) {
	registry := p.Infra.RegistryUrl
	if registry == "" {
		registry = p.Infra.ImageRepoUrl.Registry
	}
	if registry == "" {
		return dockerregistry.AuthConfig{}, fmt.Errorf("registry URL is empty; ensure 'registry_url' or 'image_repo_url' output is set")
	}
	if !strings.HasPrefix(registry, "https://") {
		registry = "https://" + registry
	}

	// Step 1: Get ARM access token from Nullstone
	armToken, err := p.Infra.ImagePusher.GetToken(ctx, policy.TokenRequestOptions{Scopes: ARMScopes})
	if err != nil {
		return dockerregistry.AuthConfig{}, fmt.Errorf("error obtaining ARM token: %w", err)
	}

	// Step 2: Exchange ARM token for ACR refresh token
	refreshToken, err := exchangeForACRRefreshToken(ctx, registry, p.Infra.ImagePusher.TenantId, armToken.Token)
	if err != nil {
		return dockerregistry.AuthConfig{}, fmt.Errorf("error exchanging ARM token for ACR refresh token: %w", err)
	}

	// Step 3: Exchange ACR refresh token for ACR access token
	accessToken, err := exchangeForACRAccessToken(ctx, registry, refreshToken)
	if err != nil {
		return dockerregistry.AuthConfig{}, fmt.Errorf("error exchanging ACR refresh token for access token: %w", err)
	}

	serverAddr := strings.TrimPrefix(registry, "https://")
	return dockerregistry.AuthConfig{
		Username:      "00000000-0000-0000-0000-000000000000",
		Password:      accessToken,
		ServerAddress: serverAddr,
	}, nil
}

type acrExchangeResponse struct {
	RefreshToken string `json:"refresh_token"`
}

type acrTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func exchangeForACRRefreshToken(ctx context.Context, registry, tenantId, armToken string) (string, error) {
	endpoint := fmt.Sprintf("%s/oauth2/exchange", strings.TrimSuffix(registry, "/"))
	service := strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://")
	data := url.Values{}
	data.Set("grant_type", "access_token")
	data.Set("service", service)
	data.Set("tenant", tenantId)
	data.Set("access_token", armToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ACR exchange returned HTTP %d", resp.StatusCode)
	}

	var result acrExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding ACR exchange response: %w", err)
	}
	return result.RefreshToken, nil
}

func exchangeForACRAccessToken(ctx context.Context, registry, refreshToken string) (string, error) {
	endpoint := fmt.Sprintf("%s/oauth2/token", strings.TrimSuffix(registry, "/"))
	service := strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://")
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("service", service)
	data.Set("scope", "repository:*:*")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ACR token endpoint returned HTTP %d", resp.StatusCode)
	}

	var result acrTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding ACR token response: %w", err)
	}
	return result.AccessToken, nil
}
