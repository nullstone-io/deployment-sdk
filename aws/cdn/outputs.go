package cdn

import (
	"github.com/nullstone-io/deployment-sdk/aws"
)

type Outputs struct {
	Region   string     `ns:"region"`
	Deployer nsaws.User `ns:"deployer"`
	CdnIds   []string   `ns:"cdn_ids"`
}
