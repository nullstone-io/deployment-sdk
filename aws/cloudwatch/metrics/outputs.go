package metrics

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

type Outputs struct {
	Region          string        `ns:"region"`
	MetricsReader   nsaws.User    `ns:"metrics_reader,optional"`
	LogReader       nsaws.User    `ns:"log_reader,optional"`
	MetricsMappings MappingGroups `ns:"metrics_mappings"`
}

func (o Outputs) AwsConfig() aws.Config {
	if o.MetricsReader.Name != "" {
		return nsaws.NewConfig(o.MetricsReader, o.Region)
	}
	return nsaws.NewConfig(o.LogReader, o.Region)
}
