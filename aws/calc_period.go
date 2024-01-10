package nsaws

import (
	"math"
	"time"
)

// CalcPeriod determines how much time between datapoints
// This result is in number of seconds that can be used against the AWS API GetMetricData
// If period is small, we collect too much data and impair performance (retrieval and render)
// Since this offers no meaningful benefit to the user, we calculate period based on the time window (end - start)
// We are aiming for 60 datapoints total (e.g. 1m period : 1h window)
// If time window results in a decimal period, we round (resulting in more than 60 datapoints, at most 29)
func CalcPeriod(start *time.Time, end *time.Time) int32 {
	s, e := time.Now().Add(-time.Hour), time.Now()
	if start != nil {
		s = *start
	}
	if end != nil {
		e = *end
	}
	window := e.Sub(s)
	return int32(math.Round(window.Hours()) * 60)
}
