package cloudmonitoring

import (
	"math"
	"time"
)

// CalcPeriod determines how much time between datapoints
// This result is in number of seconds that can be used against the GCP CloudMonitoring APIs
// If period is small, we collect too much data and impair performance (retrieval and render)
// Since this offers no meaningful benefit to the user, we calculate period based on the time window (end - start)
// We are aiming for 60 datapoints total (e.g. 1m period : 1h window)
// If time window results in a decimal period, we round (resulting in more than 60 datapoints, at most 29)
//
// This adheres to CloudMonitoring API constraints:
// - Min: 60 seconds
// - Max: 104 weeks (2 years)
func CalcPeriod(start *time.Time, end *time.Time) time.Duration {
	s, e := time.Now().Add(-time.Hour), time.Now()
	if start != nil {
		s = *start
	}
	if end != nil {
		e = *end
	}
	window := e.Sub(s)
	numSecs := int64(math.Round(window.Hours()) * 60)
	return time.Duration(numSecs) * time.Second
}
