package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/tomnomnom/linkheader"
	"io"
	"net/http"
	"net/url"
)

type ListRemoteTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func ListRemoteTags(ctx context.Context, targetUrl ImageUrl, targetAuth types.AuthConfig) ([]string, error) {
	reqUrl := (&url.URL{
		Scheme: targetUrl.Scheme(),
		Host:   targetUrl.Registry,
		Path:   fmt.Sprintf("/v2/%s/tags/list", targetUrl.RepoName()),
	}).String()

	allTags := make([]string, 0)
	for {
		pageTags, res, err := doListRemoteTags(ctx, reqUrl, targetAuth)
		if err != nil {
			return nil, err
		} else if pageTags != nil {
			allTags = append(allTags, pageTags...)
		}

		// Continue listing tags on the next page if the response contains a "next page"
		if reqUrl = getNextPageUrl(res); reqUrl == "" {
			return allTags, nil
		}
	}
}

func doListRemoteTags(ctx context.Context, reqUrl string, targetAuth types.AuthConfig) ([]string, *http.Response, error) {
	client := &http.Client{Transport: AuthedTransport{Auth: targetAuth}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating list remote tags request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting list remote tags response: %w", err)
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode >= 400 {
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, res, fmt.Errorf("error reading error (status code = %d) response: %w", res.StatusCode, err)
		}
		return nil, res, fmt.Errorf("error response (status code = %d): %s", res.StatusCode, string(raw))
	}

	decoder := json.NewDecoder(res.Body)
	var result ListRemoteTagsResponse
	if err := decoder.Decode(&result); err != nil {
		return nil, res, fmt.Errorf("error reading json response: %w", err)
	}

	return result.Tags, res, nil
}

func getNextPageUrl(res *http.Response) string {
	val := res.Header.Get("Link")
	if val == "" {
		return ""
	}
	for _, link := range linkheader.Parse(val) {
		if link.Rel == "next" {
			return link.URL
		}
	}
	return ""
}
