package prometheus

// Resources
// - https://cloud.google.com/stackdriver/docs/managed-prometheus/query#http-api-details

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"time"
)

type QueryClient struct {
	BaseUrl    *url.URL
	HttpClient *http.Client
}

type QueryOptions struct {
	Start *time.Time
	End   *time.Time
	// Step refers to the resolution of datapoints, measured in number of seconds
	Step int
}

func (c *QueryClient) Query(ctx context.Context, query string, options QueryOptions) (*QueryResponse, error) {
	u := *c.BaseUrl
	u.Path = path.Join(c.BaseUrl.Path, "api/v1/query_range")
	params := url.Values{}
	params.Add("query", query)
	if options.Start != nil {
		params.Add("start", options.Start.Format(time.RFC3339))
	}
	if options.End != nil {
		params.Add("end", options.End.Format(time.RFC3339))
	}
	params.Add("step", fmt.Sprint(options.Step))
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus query request: %w", err)
	}
	raw, err := httputil.DumpRequestOut(req, true)
	log.Println(string(raw))
	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making prometheus query request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode >= 500 {
		raw, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error response from prometheus query [%d - %s]: %s", res.StatusCode, res.Status, string(raw))
	}

	var response QueryResponse
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding prometheus query response: %w", err)
	}
	if response.Status == "error" {
		return nil, fmt.Errorf("prometheus error [%s]: %s", response.ErrorType, response.Error)
	}
	return &response, nil
}

type QueryResponse struct {
	Status    string            `json:"status"`
	Data      QueryResponseData `json:"data"`
	ErrorType string            `json:"errorType"`
	Error     string            `json:"error"`
	Warnings  []string          `json:"warnings"`
	Infos     []string          `json:"infos"`
}

type QueryResponseData struct {
	ResultType string                    `json:"resultType"`
	Result     []QueryResponseDataResult `json:"result"`
}

type QueryResponseDataResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

func (r QueryResponseDataResult) GetTimePoint(index int) (TimePoint, error) {
	if r.Values == nil {
		return TimePoint{}, fmt.Errorf("no values found")
	}
	if index < 0 || index >= len(r.Values) {
		return TimePoint{}, fmt.Errorf("no time point data for index [%d]", index)
	}
	return TimePointFromSlice(r.Values[index])
}

func TimePointFromSlice(vals []any) (TimePoint, error) {
	if len(vals) != 2 {
		return TimePoint{}, fmt.Errorf("time point data does not contain 2 items")
	}

	ts, ok := vals[0].(float64)
	if !ok {
		return TimePoint{}, fmt.Errorf("time point data does not contain a valid timestamp")
	}
	raw, ok := vals[1].(string)
	if !ok {
		return TimePoint{}, fmt.Errorf("time point data does not contain a valid scalar value")
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return TimePoint{}, fmt.Errorf("time point data does not contain a valid scalar value: %w", err)
	}

	// convert unix timestamp to time.Time
	// Separate the integer (seconds) and fractional (nanoseconds) parts
	secs := int64(ts)                          // integral seconds
	nsecs := int64((ts - float64(secs)) * 1e9) // fractional part as nanoseconds

	return TimePoint{
		Timestamp: time.Unix(secs, nsecs).UTC(),
		Value:     val,
	}, nil
}

type TimePoint struct {
	Timestamp time.Time
	Value     float64
}
