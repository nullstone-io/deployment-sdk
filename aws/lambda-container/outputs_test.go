package lambda_container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputs_VersionsToKeep(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name             string
		output           *int
		wantKeep         int
		wantUsingDefault bool
	}{
		{name: "output missing uses default", output: nil, wantKeep: DefaultVersionsToKeep, wantUsingDefault: true},
		{name: "output set", output: intPtr(5), wantKeep: 5},
		{name: "output matching default is not reported as default", output: intPtr(DefaultVersionsToKeep), wantKeep: DefaultVersionsToKeep},
		{name: "zero keeps the live version", output: intPtr(0), wantKeep: 1},
		{name: "negative keeps the live version", output: intPtr(-2), wantKeep: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outs := Outputs{LambdaVersionsToKeep: test.output}
			keep, usingDefault := outs.VersionsToKeep()
			assert.Equal(t, test.wantKeep, keep)
			assert.Equal(t, test.wantUsingDefault, usingDefault)
		})
	}
}
