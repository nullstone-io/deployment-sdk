package prometheus

import (
	"fmt"
	"math"
	"regexp"
	"time"
)

const (
	DefaultPanelWidth     = 1200
	DefaultScrapeInterval = 60
)

type IntervalOptions struct {
	Start time.Time
	End   time.Time
	// PanelWidth refers to the width of the panel, measured in pixels
	// This is used to
	PanelWidth int
	// ScrapeInterval refers to the interval at which the metric is scraped, measured in number of seconds
	ScrapeInterval int
	// MaxDataPoints refers to the maximum number of data points to plot (if 0, use PanelWidth)
	MaxDataPoints int
	// MinInterval refers to the minimum interval to use (if 0, use ScrapeInterval)
	MinInterval int
}

// IntervalCalculator handles interval calculations and query substitution
type IntervalCalculator struct {
	interval     int // calculated $__interval in seconds
	rateInterval int // calculated $__rate_interval in seconds
}

// NewIntervalCalculator creates a new calculator with the given parameters
func NewIntervalCalculator(opts IntervalOptions) *IntervalCalculator {
	calc := &IntervalCalculator{}
	calc.calculate(opts)
	return calc
}

// calculate computes $__interval and $__rate_interval
func (c *IntervalCalculator) calculate(opts IntervalOptions) {
	// Set defaults
	width := opts.PanelWidth
	if width == 0 {
		width = DefaultPanelWidth
	}

	scrapeInterval := opts.ScrapeInterval
	if scrapeInterval == 0 {
		scrapeInterval = DefaultScrapeInterval
	}

	maxDataPoints := opts.MaxDataPoints
	if maxDataPoints == 0 {
		maxDataPoints = width
	}

	minInterval := opts.MinInterval
	if minInterval == 0 {
		minInterval = scrapeInterval
	}

	// Calculate range in seconds
	rangeSeconds := int(opts.End.Sub(opts.Start).Seconds())

	// Calculate $__interval (basic interval)
	// interval = range / desired_points
	interval := rangeSeconds / maxDataPoints

	// Ensure minimum interval
	if interval < minInterval {
		interval = minInterval
	}

	c.interval = interval

	// Calculate $__rate_interval
	// rate_interval = max(interval + scrape_interval, 4 * scrape_interval)
	optionA := interval + scrapeInterval
	optionB := 4 * scrapeInterval

	c.rateInterval = int(math.Max(float64(optionA), float64(optionB)))
}

// GetInterval returns the calculated $__interval in seconds
func (c *IntervalCalculator) GetInterval() int {
	return c.interval
}

// GetRateInterval returns the calculated $__rate_interval in seconds
func (c *IntervalCalculator) GetRateInterval() int {
	return c.rateInterval
}

// GetIntervalString returns $__interval as a duration string (e.g., "60s")
func (c *IntervalCalculator) GetIntervalString() string {
	return formatDuration(c.interval)
}

// GetRateIntervalString returns $__rate_interval as a duration string
func (c *IntervalCalculator) GetRateIntervalString() string {
	return formatDuration(c.rateInterval)
}

// SubstituteVariables replaces ${__interval} and ${__rate_interval} in the query
func (c *IntervalCalculator) SubstituteVariables(query string) string {
	// Replace ${__interval} and $__interval
	query = replaceVariable(query, "__interval", c.GetIntervalString())

	// Replace ${__rate_interval} and $__rate_interval
	query = replaceVariable(query, "__rate_interval", c.GetRateIntervalString())

	return query
}

// replaceVariable replaces both ${var} and $var patterns
func replaceVariable(query, varName, value string) string {
	// Pattern 1: ${__variable}
	pattern1 := regexp.MustCompile(`\$\{` + varName + `\}`)
	query = pattern1.ReplaceAllString(query, value)

	// Pattern 2: $__variable (without braces)
	pattern2 := regexp.MustCompile(`\$` + varName + `\b`)
	query = pattern2.ReplaceAllString(query, value)

	return query
}

// formatDuration converts seconds to Prometheus duration string
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 3600 {
		minutes := seconds / 60
		remainder := seconds % 60
		if remainder == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, remainder)
	} else if seconds < 86400 {
		hours := seconds / 3600
		remainder := seconds % 3600
		if remainder == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		minutes := remainder / 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := seconds / 86400
		remainder := seconds % 86400
		if remainder == 0 {
			return fmt.Sprintf("%dd", days)
		}
		hours := remainder / 3600
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
