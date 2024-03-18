package ecs

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/display"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"k8s.io/utils/strings/slices"
	"sort"
	"strings"
	"time"
)

var (
	enabledOrgs = []string{
		"nullstone",
		"fayde",
		"BSick7",
		"ssickles",
	}
	ErrNotPrimaryDeployment = errors.New("Cancelled deployment because a newer deployment invalidated this deployment.")
	ErrInactiveDeployment   = errors.New("Cancelled deployment because it is no longer active.")
)

var _ app.DeployStatusGetter = &DeployLogger{}

type DeployLogger struct {
	OsWriters    logging.OsWriters
	Infra        Outputs
	DeploymentId string

	// Loggers
	taskLoggers deployTaskLoggers

	// Cached data
	service         *ecstypes.Service
	taskDefinition  *ecstypes.TaskDefinition
	deployment      *ecstypes.Deployment
	loadBalancers   StatusLoadBalancers
	lastSeenEventAt time.Time
}

func NewDeployLogger(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	enabled := appDetails.App != nil && slices.Contains(enabledOrgs, appDetails.App.OrgName)
	if !enabled {
		return NewDeployStatusGetter(ctx, osWriters, source, appDetails)
	}

	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return &DeployLogger{
		OsWriters: osWriters,
		Infra:     outs,
	}, nil
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

func (d *DeployLogger) GetDeployStatus(ctx context.Context, deploymentId string) (app.RolloutStatus, error) {
	d.init()

	if d.Infra.ServiceName == "" {
		d.log(LogEvent{
			Source:  d.Infra.ServiceName,
			At:      time.Now(),
			Message: `Empty or missing "service_name" output in app module. Skipping check for healthy.`,
		})
		return app.RolloutStatusComplete, nil
	}
	if deploymentId == "" {
		return app.RolloutStatusUnknown, nil
	}
	if err := d.refresh(ctx, deploymentId); err != nil {
		return app.RolloutStatusUnknown, err
	}

	if err := d.isEvicted(); err != nil {
		return app.RolloutStatusCancelled, err
	}

	switch d.deployment.RolloutState {
	case ecstypes.DeploymentRolloutStateInProgress:
		return app.RolloutStatusInProgress, nil
	case ecstypes.DeploymentRolloutStateCompleted:
		return app.RolloutStatusComplete, nil
	case ecstypes.DeploymentRolloutStateFailed:
		return app.RolloutStatusFailed, nil
	default:
		return app.RolloutStatusUnknown, nil
	}
}

func (d *DeployLogger) init() {
	if d.taskLoggers == nil {
		d.taskLoggers = deployTaskLoggers{}
	}
}

func (d *DeployLogger) refresh(ctx context.Context, deploymentId string) error {
	previous := d.service
	updated, err := GetService(ctx, d.Infra)
	if err != nil {
		return fmt.Errorf("unable to retrieve service: %w", err)
	}
	d.service = updated

	d.DeploymentId = deploymentId
	previousDeployment := d.deployment
	d.refreshDeployment(*updated, deploymentId)
	// TODO: deployment.Status != "PRIMARY" means another deployment is evicting this one
	//  Should we immediately terminate watching this deployment?
	d.logInitialEvents(previous, previousDeployment)

	if err := d.loadTaskDefinitionOnce(ctx); err != nil {
		return fmt.Errorf("unable to load task definition: %w", err)
	}
	d.initLoadBalancers(*updated)
	d.initLastSeenEvent(*updated)

	if err := d.loadBalancers.RefreshHealth(ctx, d.Infra); err != nil {
		return err
	}
	if err := d.taskLoggers.Refresh(ctx, d.OsWriters, d.Infra, deploymentId, d.loadBalancers, d.taskDefinition); err != nil {
		return err
	}

	d.logDifferences(previous, previousDeployment)
	d.logNewEvents()

	return nil
}

// isEvicted determines if the current deployment was evicted by another deployment
// This allows us to immediately cancel the deployment if another deployment becomes the primary
func (d *DeployLogger) isEvicted() error {
	switch aws.ToString(d.deployment.Status) {
	case "PRIMARY":
		// We're still the primary deployment, we haven't been evicted
		return nil
	case "ACTIVE":
		// Another deployment was created after us
		return ErrNotPrimaryDeployment
	case "INACTIVE":
		// This deployment is no longer active
		// This shouldn't happen, we will handle it if something odd happens
		return ErrInactiveDeployment
	}
	// For unknown cases, we will say that we're not evicted
	return nil
}

func (d *DeployLogger) loadTaskDefinitionOnce(ctx context.Context) error {
	// Retrieve the taskDefinition *once* to use for reference
	if d.taskDefinition == nil && d.service.TaskDefinition != nil {
		// We only need to load task definition once because we're tracking a single deployment
		var err error
		d.taskDefinition, err = GetTaskDefinitionByArn(ctx, d.Infra, *d.service.TaskDefinition)
		if err != nil {
			return fmt.Errorf("unable to retrieve task definition: %w", err)
		}
	}
	return nil
}

func (d *DeployLogger) refreshDeployment(service ecstypes.Service, deploymentId string) {
	for _, deployment := range service.Deployments {
		if aws.ToString(deployment.Id) == deploymentId {
			d.deployment = &deployment
			break
		}
	}
}

// initLoadBalancers stores load balancers once as soon as we see the deployment
//
// Logging updates from load balancers is tricky...
// Attaching/Detaching load balancers from a service causes a new deployment
// Theoretically, we could isolate our analysis to the load balancer(s) affected by the current deployment
// However, the only way to retries these attachments is by retrieving the ECS Service
// If there are new deployments while we're logging, the list load balancers will be different
//
// To address this issue, we are going to do the following:
// Capture the load balancer(s) once when we first see the deployment in Service
// Refresh the target group health on those load balancers each poll
// Provide each task logger with this information so that it can log health updates
// (For ECS, we use the Task IpAddress to find a Target in a Target Group by ID -- TargetID = IP Address)
func (d *DeployLogger) initLoadBalancers(service ecstypes.Service) {
	if d.loadBalancers != nil {
		// We already initialized load balancers
		return
	}
	if d.deployment != nil {
		// We have a deployment
		d.loadBalancers = StatusLoadBalancersFromEcsService(&service)
	}
}

// initLastSeenEvent seeds the lastSeenEventAt based on when the tracking deployment was created
// This ensures that when we log new events, we're not reporting events that don't belong to this deployment
func (d *DeployLogger) initLastSeenEvent(service ecstypes.Service) {
	if !d.lastSeenEventAt.IsZero() {
		// We have already seeded, bye bye
		return
	}

	if d.deployment != nil {
		// We have a deployment
		for _, evt := range service.Events {
			if aws.ToTime(evt.CreatedAt).After(d.lastSeenEventAt) {
				d.lastSeenEventAt = *evt.CreatedAt
			}
		}
	}
}

// logInitialEvents logs messages about initial creation of the service and deployment
func (d *DeployLogger) logInitialEvents(previous *ecstypes.Service, previousDeployment *ecstypes.Deployment) {
	now := time.Now()
	if d.deployment != nil && previousDeployment == nil {
		deployCreatedAt := aws.ToTime(d.deployment.CreatedAt)
		d.log(LogEvent{
			Source:  d.DeploymentId,
			At:      deployCreatedAt,
			Message: "Deployment created",
		})
	}
	if d.service != nil && previous == nil {
		// NOTE: This is here for consistency
		// I don't know if there is something interesting to log when we first identify the service
		if period := d.service.HealthCheckGracePeriodSeconds; period != nil && *period > 0 {
			d.log(LogEvent{
				Source:  d.Infra.ServiceName,
				At:      now,
				Message: fmt.Sprintf("Service tasks have a health check grace period of %s", time.Second*time.Duration(*period)),
			})
		}
	}
}

// logDifferences compares previous service and deployment to current service and deployment
// Differences are logged as user-friendly messages (they read as actions performed by AWS)
func (d *DeployLogger) logDifferences(previous *ecstypes.Service, previousDeployment *ecstypes.Deployment) {
	if d.service != nil && previous != nil {
		// NOTE: I don't know of anything noteworthy to log yet
	}
	if d.deployment != nil && previousDeployment != nil {
		createdAt := aws.ToTime(d.deployment.CreatedAt)
		updatedAt := aws.ToTime(d.deployment.UpdatedAt)
		if d.deployment.DesiredCount != previousDeployment.DesiredCount {
			d.log(LogEvent{
				Source:  d.DeploymentId,
				At:      createdAt,
				Message: fmt.Sprintf("Launching %d tasks", d.deployment.DesiredCount),
			})
		}
		if d.deployment.RunningCount != previousDeployment.RunningCount {
			d.log(LogEvent{
				Source:  d.DeploymentId,
				At:      updatedAt,
				Message: fmt.Sprintf("Deployment has %d running tasks", d.deployment.RunningCount),
			})
		}
		if deployStatus := aws.ToString(d.deployment.Status); deployStatus != aws.ToString(previousDeployment.Status) {
			d.log(LogEvent{
				Source:  d.DeploymentId,
				At:      updatedAt,
				Message: fmt.Sprintf("Deployment transitioned to %s", deployStatus),
			})
		}
	}
}

// logNewEvents find events in the service.Events that we haven't logged
// They are immediately logged
func (d *DeployLogger) logNewEvents() {
	since := d.lastSeenEventAt
	newEvents := make([]LogEvent, 0)
	for _, evt := range d.service.Events {
		if evt.CreatedAt != nil && evt.CreatedAt.After(since) {
			// "events" are usually sorted newest to oldest
			// we're only updating "latest" if this event is newer
			newEvents = append(newEvents, LogEvent{
				Source:  d.Infra.ServiceName,
				At:      aws.ToTime(evt.CreatedAt),
				Message: aws.ToString(evt.Message),
			})
			if evt.CreatedAt.After(d.lastSeenEventAt) {
				d.lastSeenEventAt = *evt.CreatedAt
			}
		}
	}
	sort.SliceStable(newEvents, func(i, j int) bool {
		return newEvents[i].At.Before(newEvents[j].At)
	})

	svcEventPrefix := fmt.Sprintf("(service %s)", d.Infra.ServiceName)
	deployEventPrefix := fmt.Sprintf("(service %s) (deployment %s)", d.Infra.ServiceName, d.DeploymentId)
	for _, evt := range newEvents {
		if strings.Contains(evt.Message, deployEventPrefix) {
			evt.Source = d.DeploymentId
			evt.Message = strings.Replace(evt.Message, deployEventPrefix, "Deployment", 1)
			evt.Message = strings.Replace(evt.Message, "Deployment deployment", "Deployment", 1) // Remove duplicate "deployment"
		} else {
			evt.Message = strings.Replace(evt.Message, svcEventPrefix, "Service", 1)
		}
		d.log(evt)
	}
}

func (d *DeployLogger) log(evt LogEvent) {
	fmt.Fprintln(d.OsWriters.Stdout(), evt)
}
