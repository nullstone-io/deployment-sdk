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
