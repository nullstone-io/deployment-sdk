package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type MetricsGetter interface {
	GetMetrics(ctx context.Context, options MetricsGetterOptions) (*MetricsData, error)
}

type MetricsGetterOptions struct {
	StartTime *time.Time
	EndTime   *time.Time

	// PanelWidth refers to the width of the panel, measured in pixels
	// This is used to calculate the number of data points to plot
	PanelWidth int
	// ScrapeInterval is how often metrics are collected from targets, in seconds.
	// Determines the minimum window size for rate() to have enough data points (default: 60).
	ScrapeInterval int

	Metrics []string
}

func IsMetricsNotSupported(err error) (MetricsNotSupportedError, bool) {
	var mnse MetricsNotSupportedError
	if errors.As(err, &mnse) {
		return mnse, true
	}
	return mnse, false
}

var _ error = MetricsNotSupportedError{}

type MetricsNotSupportedError struct {
	InnerErr error
}

func (e MetricsNotSupportedError) Error() string {
	return fmt.Sprintf("metrics not supported: %s", e.InnerErr)
}

func (e MetricsNotSupportedError) Unwrap() error {
	return e.InnerErr
}
