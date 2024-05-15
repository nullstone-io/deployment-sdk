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
	four := int32(4)
	five := int32(5)
	six := int32(6)
	seven := int32(7)
	eight := int32(8)
	nine := int32(9)
	ten := int32(10)
	eleven := int32(11)

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
				{Revision: &eight},
				{Revision: &nine},
				{Revision: &ten},
				{Revision: &one},
				{Revision: &three},
				{Revision: &two},
				{Revision: &eleven},
				{Revision: &four},
				{Revision: &five},
				{Revision: &six},
				{Revision: &seven},
			},
			want: &eleven,
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
