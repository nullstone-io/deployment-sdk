package batch

import (
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetJobDefinition(t *testing.T) {
	one := int32(1)
	two := int32(2)
	three := int32(3)

	tests := []struct {
		name string
		defs []batchtypes.JobDefinition
		want *int32
	}{
		{
			name: "no definitions",
			defs: []batchtypes.JobDefinition{},
			want: nil,
		},
		{
			name: "single definition",
			defs: []batchtypes.JobDefinition{
				{Revision: &one},
			},
			want: &one,
		},
		{
			name: "multiple definitions",
			defs: []batchtypes.JobDefinition{
				{Revision: &one},
				{Revision: &three},
				{Revision: &two},
			},
			want: &three,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			latest := getLatestJobDefinition(test.defs)
			var latestRevision *int32
			if latest != nil {
				latestRevision = latest.Revision
			}
			assert.Equal(t, test.want, latestRevision)
		})
	}
}
