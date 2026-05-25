package cloudrun

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/nullstone-io/deployment-sdk/docker"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s Statuser) statusService(ctx context.Context) (*Service, error) {
	svcClient, err := NewServicesClient(ctx, s.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run services client: %w", err)
	}
	defer svcClient.Close()

	svc, err := svcClient.GetService(ctx, &runpb.GetServiceRequest{Name: s.Infra.ServiceId})
	if err != nil {
		return nil, fmt.Errorf("error retrieving service: %w", err)
	} else if svc == nil {
		return nil, fmt.Errorf("cloud run service %q not found", s.Infra.ServiceName)
	}

	// Index the observed traffic split by revision name.
	trafficByRev := map[string]*runpb.TrafficTargetStatus{}
	taggedUrls := make([]RevisionTag, 0)
	for _, tt := range svc.GetTrafficStatuses() {
		trafficByRev[tt.GetRevision()] = tt
		if tt.GetTag() != "" && tt.GetUri() != "" {
			taggedUrls = append(taggedUrls, RevisionTag{Name: tt.GetTag(), Url: tt.GetUri()})
		}
	}

	revClient, err := NewRevisionsClient(ctx, s.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run revisions client: %w", err)
	}
	defer revClient.Close()

	revisions := make([]Revision, 0)
	it := revClient.ListRevisions(ctx, &runpb.ListRevisionsRequest{Parent: s.Infra.ServiceId})
	for {
		rev, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("error listing revisions: %w", err)
		}
		revisions = append(revisions, s.mapRevision(rev, svc, trafficByRev))
	}

	state := ServiceStateHealthy
	for _, rev := range revisions {
		if rev.Role == RevisionRoleFailed {
			state = ServiceStateFailing
			break
		}
	}

	return &Service{
		ServiceName:    s.Infra.ServiceName,
		Generation:     svc.GetGeneration(),
		State:          state,
		Url:            svc.GetUri(),
		TaggedUrls:     taggedUrls,
		LastDeployedAt: tsToTime(svc.GetUpdateTime()),
		Revisions:      revisions,
	}, nil
}

func (s Statuser) mapRevision(rev *runpb.Revision, svc *runpb.Service, trafficByRev map[string]*runpb.TrafficTargetStatus) Revision {
	name := shortName(rev.GetName())
	tt := trafficByRev[name]
	var traffic int32
	var tag *RevisionTag
	if tt != nil {
		traffic = tt.GetPercent()
		if tt.GetTag() != "" {
			tag = &RevisionTag{Name: tt.GetTag(), Url: tt.GetUri()}
		}
	}

	out := Revision{
		Name:           name,
		Label:          deriveRevisionLabel(name, svc.GetName()),
		CreatedAt:      tsToTime(rev.GetCreateTime()),
		TrafficPercent: traffic,
		Tag:            tag,
		Concurrency:    rev.GetMaxInstanceRequestConcurrency(),
		Instances:      make([]Instance, 0),
	}
	if scaling := rev.GetScaling(); scaling != nil {
		out.MinInstances = scaling.GetMinInstanceCount()
		out.MaxInstances = scaling.GetMaxInstanceCount()
	}
	// Phase 1: the Run v2 API exposes only the desired minimum, not the live
	// warm-instance count. Live counts + instance cells arrive in Phase 2 via
	// Cloud Monitoring.
	if ss := rev.GetScalingStatus(); ss != nil {
		out.InstanceCount = ss.GetDesiredMinInstanceCount()
	}

	_, container := GetContainerByName(rev.GetContainers(), s.Infra.MainContainerName)
	if container == nil && len(rev.GetContainers()) > 0 {
		container = rev.GetContainers()[0]
	}
	if container != nil {
		img := docker.ParseImageUrl(container.GetImage())
		out.AppVersion = img.Tag
		if limits := container.GetResources().GetLimits(); limits != nil {
			out.Cpu = limits["cpu"]
			out.Memory = limits["memory"]
		}
	}

	out.Role = s.deriveRevisionRole(rev, svc, traffic, tag)
	out.Failure = s.deriveRevisionFailure(rev, out.Role)
	return out
}

func (s Statuser) deriveRevisionRole(rev *runpb.Revision, svc *runpb.Service, traffic int32, tag *RevisionTag) RevisionRole {
	name := shortName(rev.GetName())
	isLatestCreated := name == shortName(svc.GetLatestCreatedRevision())

	if readyConditionFailed(rev.GetConditions()) {
		return RevisionRoleFailed
	}
	if isLatestCreated {
		if traffic == 0 {
			// Deployed and healthy, but traffic is pinned to a prior revision.
			return RevisionRoleStuck
		}
		return RevisionRoleLatest
	}
	if traffic > 0 {
		return RevisionRolePrior
	}
	if tag != nil {
		return RevisionRoleTagged
	}
	return RevisionRoleIdle
}

func (s Statuser) deriveRevisionFailure(rev *runpb.Revision, role RevisionRole) *Failure {
	switch role {
	case RevisionRoleFailed:
		msg := readyConditionMessage(rev.GetConditions())
		if msg == "" {
			msg = "Container failed to start. Verify the container listens on the port defined by $PORT."
		}
		return &Failure{
			Code:    FailureContainerFailedToStart,
			Title:   "Container failed to start",
			Message: msg,
		}
	case RevisionRoleStuck:
		return &Failure{
			Code:    FailureStuckAtZeroTraffic,
			Title:   "Deployed, but receiving 0% traffic",
			Message: "The latest revision is healthy but traffic is pinned to a prior revision. Promote it manually to route traffic.",
		}
	}
	return nil
}

// readyConditionFailed reports whether the revision's Ready condition is in a
// failed state.
func readyConditionFailed(conditions []*runpb.Condition) bool {
	for _, c := range conditions {
		if c.GetType() == "Ready" {
			return c.GetState() == runpb.Condition_CONDITION_FAILED
		}
	}
	return false
}

func readyConditionMessage(conditions []*runpb.Condition) string {
	for _, c := range conditions {
		if c.GetType() == "Ready" {
			return c.GetMessage()
		}
	}
	return ""
}

// shortName returns the final path segment of a resource name, e.g.
// "projects/p/locations/r/services/s/revisions/s-00012-abc" -> "s-00012-abc".
func shortName(resourceName string) string {
	if resourceName == "" {
		return ""
	}
	idx := strings.LastIndex(resourceName, "/")
	if idx < 0 {
		return resourceName
	}
	return resourceName[idx+1:]
}

// deriveRevisionLabel turns a Cloud Run revision name into a friendly label.
// Revision names follow "{service}-{NNNNN}-{hash}"; we surface "rev {N}".
func deriveRevisionLabel(revisionName, serviceName string) string {
	suffix := strings.TrimPrefix(revisionName, shortName(serviceName)+"-")
	parts := strings.Split(suffix, "-")
	if len(parts) > 0 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			return fmt.Sprintf("rev %d", n)
		}
	}
	return revisionName
}

// tsToTime converts a protobuf timestamp to *time.Time, returning nil for an
// absent or zero timestamp.
func tsToTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	if t.IsZero() {
		return nil
	}
	return &t
}
