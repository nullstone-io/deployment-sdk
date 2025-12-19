package cloudrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/display"
	"github.com/nullstone-io/deployment-sdk/logging"
)

var _ app.DeployStatusGetter = &JobDeployLogger{}

type JobDeployLogger struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d *JobDeployLogger) Close() {}

func (d *JobDeployLogger) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if d.Infra.JobName == "" {
		d.log(LogEvent{
			Source:  d.Infra.ServiceName,
			At:      time.Now(),
			Message: `Empty or missing "job_name" output in app module. Skipping check for healthy.`,
		})
		return app.RolloutStatusComplete, nil
	}

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	client, err := NewJobsClient(ctx, d.Infra.Deployer)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}

	op := client.UpdateJobOperation(reference)
	_, err = op.Poll(ctx)
	if op.Done() {
		if err != nil {
			return app.RolloutStatusFailed, err
		} else {
			return app.RolloutStatusComplete, nil
		}
	}
	return app.RolloutStatusInProgress, nil
}

func (d *JobDeployLogger) log(evt LogEvent) {
	fmt.Fprintln(d.OsWriters.Stdout(), evt)
}

type LogEvent struct {
	Source  string
	At      time.Time
	Message string
}

func (e LogEvent) String() string {
	padding := ""
	if len(e.Source) < 32 {
		padding = strings.Repeat(" ", 32-len(e.Source))
	}
	src := fmt.Sprintf("[%s]%s", e.Source, padding)
	return fmt.Sprintf("%s %s %s", display.FormatTime(e.At), src, e.Message)
}
