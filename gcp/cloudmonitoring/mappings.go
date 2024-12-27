package cloudmonitoring

import (
	"bytes"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"time"
)

type MappingGroups []MappingGroup

type MappingGroup struct {
	Name     string                      `json:"name"`
	Type     workspace.MetricDatasetType `json:"type"`
	Unit     string                      `json:"unit"`
	Mappings map[string]MetricMapping    `json:"mappings"`
}

// MetricMapping is retrieved from a workspace's outputs and can be used to construct an MQL query
//
// Example:
// fetch k8s_container
// | metric 'kubernetes.io/container/cpu/request_utilization'
// | filter <resource_filter>
// | group_by <group_by>
// | <expression>
// | every <interval>
// | from "<start>" to "<end>"
type MetricMapping struct {
	ProjectId string `json:"project_id"`
	// MetricContext chooses the context over which to grab a metric
	// A common value is "k8s_container" to fetch metrics across the kubernetes containers
	MetricContext  string   `json:"metric_context"`
	MetricName     string   `json:"metric_name"`
	ResourceFilter string   `json:"resource_filter"`
	Filters        []string `json:"filters"`
	GroupBy        string   `json:"group_by"`
	// Expression can be used to adjust the values on the way out
	// This will not be used if it's empty
	// A common use case is create a percentage
	// An expression of "value * 100" would shift a set of values in range 0-1 to 0-100
	Expression string `json:"expression"`
}

func (m MetricMapping) ConstructMQL(interval time.Duration, start time.Time, end time.Time) string {
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("fetch %s", m.MetricContext))
	buf.WriteString(fmt.Sprintf("\n| metric '%s'", m.MetricName))
	for _, filter := range m.Filters {
		buf.WriteString(fmt.Sprintf("\n| filter %s", filter))
	}
	//buf.WriteString(fmt.Sprintf("\n| filter (%s)", strings.Replace(m.ResourceFilter, "\n", " ", -1)))
	if m.GroupBy != "" {
		buf.WriteString(fmt.Sprintf("\n| group_by %s", m.GroupBy))
	}
	buf.WriteString(fmt.Sprintf("\n| every %s", durationToMQL(interval)))
	buf.WriteString(fmt.Sprintf("\n| from %q to %q", start.Format(time.RFC3339), end.Format(time.RFC3339)))
	return buf.String()
}

// durationToMQL serializes a time.Duration into an "every <interval>" string that is valid for MQL
// In MQL, the | every <duration> clause expects a string like "1m", "5s", or "2h30m",
// not Go’s default String() format (which might produce something like "2m0s").
// MQL is fairly flexible but can fail if it sees certain patterns (like "0s" in places you don’t expect).
func durationToMQL(d time.Duration) string {
	// Break down duration into hours, minutes, seconds.
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	result := ""
	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 {
		result += fmt.Sprintf("%ds", seconds)
	}
	// If duration was 0 (or less than 1s), produce "0s" so MQL doesn't choke on empty string.
	if result == "" {
		result = "0s"
	}
	return result
}
