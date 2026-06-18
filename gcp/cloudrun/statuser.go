package cloudrun

import (
	"context"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

var (
	_ app.StatusOverviewResult = StatusOverview{}
	_ app.Statuser             = Statuser{}
)

func NewStatuser(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Statuser, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return Statuser{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Statuser struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (s Statuser) StatusOverview(ctx context.Context) (app.StatusOverviewResult, error) {
	ov := StatusOverview{
		Location:     s.Infra.Location(),
		ServiceName:  s.Infra.ServiceName(),
		JobName:      s.Infra.JobName(),
		TrafficSplit: make([]TrafficSplitEntry, 0),
	}

	if s.Infra.ServiceId != "" {
		svc, err := s.statusService(ctx)
		if err != nil {
			return ov, err
		}
		versions := make([]string, 0)
		for _, rev := range svc.Revisions {
			if rev.TrafficPercent <= 0 && rev.Role != RevisionRoleLatest {
				continue
			}
			ov.TrafficSplit = append(ov.TrafficSplit, TrafficSplitEntry{
				RevisionName:   rev.Name,
				TrafficPercent: rev.TrafficPercent,
			})
			if rev.TrafficPercent > 0 {
				ov.ServingRevisionCount++
			}
			if rev.AppVersion != "" {
				versions = append(versions, rev.AppVersion)
			}
		}
		ov.versions = versions
		return ov, nil
	}

	execs, err := s.statusJob(ctx)
	if err != nil {
		return ov, err
	}
	if len(execs) > 0 && execs[0].AppVersion != "" {
		ov.versions = []string{execs[0].AppVersion}
	}
	return ov, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	out := Status{
		Location:    s.Infra.Location(),
		ServiceName: s.Infra.ServiceName(),
		JobName:     s.Infra.JobName(),
		Executions:  make([]JobExecution, 0),
	}

	if s.Infra.ServiceId != "" {
		svc, err := s.statusService(ctx)
		if err != nil {
			return nil, err
		}
		// Best-effort: layer live instance counts + request health from Cloud
		// Monitoring on top of the Run v2 data. Failures are logged, not fatal.
		s.enrichServiceMetrics(ctx, svc)
		out.Service = svc
		return out, nil
	}

	execs, err := s.statusJob(ctx)
	if err != nil {
		return nil, err
	}
	out.Executions = execs
	return out, nil
}
