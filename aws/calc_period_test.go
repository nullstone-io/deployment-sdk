package nsaws

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCalcPeriod(t *testing.T) {
	now := time.Now()

	tests := []struct {
		start          time.Time
		end            time.Time
		wantPeriodSec  int32
		wantDatapoints int32
	}{
		{
			start:         now.Add(-time.Hour),
			end:           now,
			wantPeriodSec: 60,
		},
		{
			start:         now.Add(-3 * time.Hour),
			end:           now,
			wantPeriodSec: 60 * 3,
		},
		{
			start:         now.Add(-12 * time.Hour),
			end:           now,
			wantPeriodSec: 12 * 60,
		},
		{
			start:         now.Add(-12*time.Hour - 15*time.Minute),
			end:           now,
			wantPeriodSec: 12 * 60,
		},
		{
			start:         now.Add(-12*time.Hour - 45*time.Minute),
			end:           now,
			wantPeriodSec: 13 * 60,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			gotPeriodSec := CalcPeriod(&test.start, &test.end)
			assert.Equal(t, test.wantPeriodSec, gotPeriodSec)
		})
	}
}
