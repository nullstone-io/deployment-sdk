package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/display"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"log"
	"sort"
	"strings"
	"time"
)

var _ app.DeployStatusGetter = &DeployLogger{}

type DeployLogger struct {
	OsWriters logging.OsWriters
	Infra     Outputs

	// Loggers
	serviceLogger *log.Logger
	deployLogger  *log.Logger
	taskLoggers   deployTaskLoggers

	// Cached data
	service         *ecstypes.Service
	taskDefinition  *ecstypes.TaskDefinition
	deployment      *ecstypes.Deployment
	loadBalancers   StatusLoadBalancers
	lastSeenEventAt time.Time
}

func NewDeployLogger(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
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
	Id      string
	At      time.Time
	Message string
}

func (e LogEvent) String() string {
	return fmt.Sprintf("[%s] %s", display.FormatTime(e.At), e.Message)
}

func (d *DeployLogger) GetDeployStatus(ctx context.Context, deploymentId string) (app.RolloutStatus, error) {
	d.init()

	if d.Infra.ServiceName == "" {
		d.serviceLogger.Println(`Empty or missing "service_name" output in app module. Skipping check for healthy.`)
		return app.RolloutStatusComplete, nil
	}
	if deploymentId == "" {
		return app.RolloutStatusUnknown, nil
	}
	if err := d.refresh(ctx, deploymentId); err != nil {
		return app.RolloutStatusUnknown, err
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
	if d.serviceLogger == nil {
		d.serviceLogger = log.New(d.OsWriters.Stdout(), "", 0)
	}
	if d.deployLogger == nil {
		d.deployLogger = log.New(d.OsWriters.Stdout(), "", 0)
	}
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
	d.serviceLogger.SetPrefix(fmt.Sprintf("[%s] ", *d.service.ServiceName))

	d.deployLogger.SetPrefix(fmt.Sprintf("[%s] ", deploymentId))
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
		d.deployLogger.Println(LogEvent{
			At:      deployCreatedAt,
			Message: "Deployment created",
		})
	}
	if d.service != nil && previous == nil {
		// NOTE: This is here for consistency
		// I don't know if there is something interesting to log when we first identify the service
		if period := d.service.HealthCheckGracePeriodSeconds; period != nil && *period > 0 {
			d.serviceLogger.Println(LogEvent{
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
			d.deployLogger.Println(LogEvent{
				At:      createdAt,
				Message: fmt.Sprintf("Launching %d tasks", d.deployment.DesiredCount),
			})
		}
		if d.deployment.RunningCount != previousDeployment.RunningCount {
			d.deployLogger.Println(LogEvent{
				At:      updatedAt,
				Message: fmt.Sprintf("Deployment has %d running tasks", d.deployment.RunningCount),
			})
		}
		if deployStatus := aws.ToString(d.deployment.Status); deployStatus != aws.ToString(previousDeployment.Status) {
			d.deployLogger.Println(LogEvent{
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
				Id:      aws.ToString(evt.Id),
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

	svcEventPrefix := fmt.Sprintf("(service %s)", *d.service.ServiceName)
	deployEventPrefix := fmt.Sprintf("(service %s) (deployment %s)", *d.service.ServiceName, *d.deployment.Id)

	for _, evt := range newEvents {
		if strings.Contains(evt.Message, deployEventPrefix) {
			evt.Message = strings.Replace(evt.Message, deployEventPrefix, "Deployment", 1)
			evt.Message = strings.Replace(evt.Message, "Deployment deployment", "Deployment", 1) // Remove duplicate "deployment"
			d.deployLogger.Println(evt)
		} else {
			evt.Message = strings.Replace(evt.Message, svcEventPrefix, "Service", 1)
			d.serviceLogger.Println(evt)
		}
	}
}
