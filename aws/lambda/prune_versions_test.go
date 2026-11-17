package lambda

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsToPrune(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		keep     int
		want     []string
	}{
		{
			name:     "no published versions",
			versions: []string{"$LATEST"},
			keep:     2,
			want:     nil,
		},
		{
			name:     "fewer versions than keep",
			versions: []string{"$LATEST", "1"},
			keep:     2,
			want:     nil,
		},
		{
			name:     "exactly keep",
			versions: []string{"$LATEST", "1", "2"},
			keep:     2,
			want:     nil,
		},
		{
			name:     "prunes oldest",
			versions: []string{"$LATEST", "1", "2", "3", "4"},
			keep:     2,
			want:     []string{"1", "2"},
		},
		{
			name:     "orders numerically, not lexically",
			versions: []string{"10", "9", "$LATEST", "100", "8"},
			keep:     2,
			want:     []string{"8", "9"},
		},
		{
			name:     "keep zero prunes every published version but never $LATEST",
			versions: []string{"$LATEST", "3", "7"},
			keep:     0,
			want:     []string{"3", "7"},
		},
		{
			name:     "negative keep behaves like zero",
			versions: []string{"$LATEST", "3"},
			keep:     -1,
			want:     []string{"3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := versionsToPrune(test.versions, test.keep)
			assert.Equal(t, test.want, got)
		})
	}
}
