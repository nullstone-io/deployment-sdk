package cloudrun

import (
	"strings"

	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	ProjectId         string             `ns:"project_id,optional"`
	Region            string             `ns:"region,optional"`
	ServiceId         string             `ns:"service_id,optional"`
	ServiceName       string             `ns:"service_name,optional"`
	JobId             string             `ns:"job_id,optional"`
	JobName           string             `ns:"job_name,optional"`
	ImageRepoUrl      docker.ImageUrl    `ns:"image_repo_url,optional"`
	Deployer          gcp.ServiceAccount `ns:"deployer"`
	MainContainerName string             `ns:"main_container_name,optional"`
}

// Location returns the project and region for this workspace. When the
// project_id/region outputs are absent, it falls back to parsing them from the
// service_id/job_id, which use the form
// projects/{project}/locations/{region}/{services|jobs}/{name}.
func (o Outputs) Location() LocationInfo {
	loc := LocationInfo{ProjectId: o.ProjectId, Region: o.Region}
	if loc.ProjectId != "" && loc.Region != "" {
		return loc
	}
	id := o.ServiceId
	if id == "" {
		id = o.JobId
	}
	parts := strings.Split(id, "/")
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "projects":
			if loc.ProjectId == "" {
				loc.ProjectId = parts[i+1]
			}
		case "locations":
			if loc.Region == "" {
				loc.Region = parts[i+1]
			}
		}
	}
	return loc
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.BlockId, ws.EnvId, types.AutomationPurposeDeploy, "deployer")
}
