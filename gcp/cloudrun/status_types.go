package cloudrun

import "time"

// LocationInfo identifies where a Cloud Run resource lives. Used by the
// frontend to build GCP console deep-links.
type LocationInfo struct {
	ProjectId string `json:"projectId"`
	Region    string `json:"region"`
}

// RevisionRole classifies a revision relative to the current traffic split.
type RevisionRole string

const (
	RevisionRoleLatest RevisionRole = "Latest"
	RevisionRolePrior  RevisionRole = "Prior"
	RevisionRoleStuck  RevisionRole = "Stuck"
	RevisionRoleTagged RevisionRole = "Tagged"
	RevisionRoleFailed RevisionRole = "Failed"
	RevisionRoleIdle   RevisionRole = "Idle"
)

// InstanceState describes a single Cloud Run instance. Populated in Phase 2
// (Cloud Monitoring); empty in Phase 1.
type InstanceState string

const (
	InstanceStateWarm     InstanceState = "Warm"
	InstanceStateStarting InstanceState = "Starting"
	InstanceStateCold     InstanceState = "Cold"
	InstanceStateFailed   InstanceState = "Failed"
	InstanceStateIdle     InstanceState = "Idle"
)

type Instance struct {
	Id    string        `json:"id"`
	State InstanceState `json:"state"`
}

type RevisionTag struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

// FailureCode is a display label for a known failure mode. These are not
// authoritative codes from any catalog — they map observed conditions to the
// banners the frontend renders. FM-CR-* are Cloud Run service modes; FM-11 is
// the shared OOMKilled label reused from the K8s job card.
type FailureCode string

const (
	FailureContainerFailedToStart FailureCode = "FM-CR-01"
	FailureImagePullFailed        FailureCode = "FM-CR-02"
	FailureStuckAtZeroTraffic     FailureCode = "FM-CR-03"
	FailureErrorRateSpike         FailureCode = "FM-CR-04"
	FailureThrottledAtMax         FailureCode = "FM-CR-05"
	FailureColdStartSpike         FailureCode = "FM-CR-06"
	FailureOOMKilled              FailureCode = "FM-11"
	FailureJobTaskFailed          FailureCode = "FM-CRJ-01"
)

type Failure struct {
	Code    FailureCode `json:"code"`
	Title   string      `json:"title"`
	Message string      `json:"message"`
}

type Revision struct {
	Name              string       `json:"name"`
	Label             string       `json:"label"`
	AppVersion        string       `json:"appVersion,omitempty"`
	Sha               string       `json:"sha,omitempty"`
	CreatedAt         *time.Time   `json:"createdAt"`
	Role              RevisionRole `json:"role"`
	TrafficPercent    int32        `json:"trafficPercent"`
	InstanceCount     int32        `json:"instanceCount"`
	MinInstances      int32        `json:"minInstances"`
	MaxInstances      int32        `json:"maxInstances"`
	Concurrency       int32        `json:"concurrency"`
	Cpu               string       `json:"cpu"`
	Memory            string       `json:"memory"`
	Instances         []Instance   `json:"instances"`
	Tag               *RevisionTag `json:"tag,omitempty"`
	RequestsPerSecond *float64     `json:"requestsPerSecond,omitempty"`
	P50Ms             *float64     `json:"p50Ms,omitempty"`
	P95Ms             *float64     `json:"p95Ms,omitempty"`
	ErrorRatePercent  *float64     `json:"errorRatePercent,omitempty"`
	Failure           *Failure     `json:"failure,omitempty"`
}

type ServiceState string

const (
	ServiceStateHealthy  ServiceState = "Healthy"
	ServiceStateScaling  ServiceState = "Scaling"
	ServiceStateCold     ServiceState = "Cold"
	ServiceStateFailing  ServiceState = "Failing"
	ServiceStateDegraded ServiceState = "Degraded"
)

type RequestHealth struct {
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	P50Ms             float64 `json:"p50Ms"`
	P95Ms             float64 `json:"p95Ms"`
	ErrorRatePercent  float64 `json:"errorRatePercent"`
}

type Service struct {
	ServiceName    string         `json:"serviceName"`
	Generation     int64          `json:"generation,omitempty"`
	State          ServiceState   `json:"state"`
	Url            string         `json:"url"`
	TaggedUrls     []RevisionTag  `json:"taggedUrls"`
	LastDeployedAt *time.Time     `json:"lastDeployedAt,omitempty"`
	RequestHealth  *RequestHealth `json:"requestHealth,omitempty"`
	Revisions      []Revision     `json:"revisions"`
}

type TaskState string

const (
	TaskStateQueued    TaskState = "Queued"
	TaskStateRunning   TaskState = "Running"
	TaskStateSucceeded TaskState = "Succeeded"
	TaskStateFailed    TaskState = "Failed"
	TaskStateRetrying  TaskState = "Retrying"
	TaskStateSkipped   TaskState = "Skipped"
)

type Task struct {
	Index    int32     `json:"index"`
	State    TaskState `json:"state"`
	Attempts int32     `json:"attempts"`
}

type JobExecutionPhase string

const (
	JobExecutionPhaseQueued    JobExecutionPhase = "Queued"
	JobExecutionPhaseRunning   JobExecutionPhase = "Running"
	JobExecutionPhaseSucceeded JobExecutionPhase = "Succeeded"
	JobExecutionPhaseFailed    JobExecutionPhase = "Failed"
	JobExecutionPhaseCancelled JobExecutionPhase = "Cancelled"
)

type JobExecution struct {
	Name           string            `json:"name"`
	ExecutionId    string            `json:"executionId"`
	AppVersion     string            `json:"appVersion,omitempty"`
	Sha            string            `json:"sha,omitempty"`
	Phase          JobExecutionPhase `json:"phase"`
	ScheduledAt    *time.Time        `json:"scheduledAt,omitempty"`
	StartedAt      *time.Time        `json:"startedAt,omitempty"`
	CompletedAt    *time.Time        `json:"completedAt,omitempty"`
	ScheduledBy    string            `json:"scheduledBy,omitempty"`
	TaskCount      int32             `json:"taskCount"`
	Parallelism    int32             `json:"parallelism"`
	CompletedCount int32             `json:"completedCount"`
	RunningCount   int32             `json:"runningCount"`
	FailedCount    int32             `json:"failedCount"`
	RetryCount     int32             `json:"retryCount"`
	MaxRetries     int32             `json:"maxRetries"`
	Tasks          []Task            `json:"tasks"`
	Failure        *Failure          `json:"failure,omitempty"`
}

// Status is the full app status payload (the `data` field on the frontend
// AppStatusResult). A service workspace populates Service; a job workspace
// populates Executions.
type Status struct {
	Location    LocationInfo   `json:"location"`
	ServiceName string         `json:"serviceName"`
	JobName     string         `json:"jobName"`
	Service     *Service       `json:"service,omitempty"`
	Executions  []JobExecution `json:"executions"`
}

type TrafficSplitEntry struct {
	RevisionName   string `json:"revisionName"`
	TrafficPercent int32  `json:"trafficPercent"`
}

// StatusOverview is the lightweight `overview` field on the frontend
// AppStatusResult and implements app.StatusOverviewResult.
type StatusOverview struct {
	Location             LocationInfo        `json:"location"`
	ServiceName          string              `json:"serviceName"`
	JobName              string              `json:"jobName"`
	ServingRevisionCount int                 `json:"servingRevisionCount"`
	TrafficSplit         []TrafficSplitEntry `json:"trafficSplit"`
	versions             []string
}

func (s StatusOverview) GetDeploymentVersions() []string {
	if s.versions == nil {
		return make([]string, 0)
	}
	return s.versions
}
