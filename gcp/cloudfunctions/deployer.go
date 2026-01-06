package cloudfunctions

import (
	"context"
	"fmt"

	"cloud.google.com/go/functions/apiv1/functionspb"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return Deployer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Print() {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved Cloud Functions service outputs")
	fmt.Fprintf(stdout, "\tfunction_name:   %s\n", d.Infra.FunctionName)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	d.Print()

	if meta.Version == "" {
		return "", fmt.Errorf("no version specified, version is required to deploy")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)

	// Create Cloud Functions client
	client, err := NewCloudFunctionsClient(ctx, d.Infra.Deployer)
	if err != nil {
		return "", fmt.Errorf("error creating Cloud Functions client: %w", err)
	}
	defer client.Close()

	// Get the existing function
	function, err := client.GetFunction(ctx, &functionspb.GetFunctionRequest{
		Name: d.Infra.FunctionName,
	})
	if err != nil {
		return "", fmt.Errorf("error getting Cloud Function: %w", err)
	}

	// Update source code version and replace env vars
	SetSourceVersion(function, d.Infra.ArtifactsBucketName, d.Infra.ArtifactsKey(meta.Version))
	ReplaceEnvVars(function, env_vars.GetStandard(meta))

	// Perform update
	fmt.Fprintf(stdout, "Updating job with new application version (%s) and environment variables...\n", meta.Version)
	updateMask := &fieldmaskpb.FieldMask{Paths: []string{"sourceArchiveUrl", "environmentVariables"}}
	op, err := client.UpdateFunction(ctx, &functionspb.UpdateFunctionRequest{
		Function:   function,
		UpdateMask: updateMask,
	})
	if err != nil {
		return "", fmt.Errorf("error updating Cloud Function: %w", err)
	}
	return op.Name(), nil
}

func SetSourceVersion(function *functionspb.CloudFunction, bucketName, objectKey string) {
	function.SourceCode = &functionspb.CloudFunction_SourceArchiveUrl{
		SourceArchiveUrl: fmt.Sprintf("gs://%s/%s", bucketName, objectKey),
	}
}

func ReplaceEnvVars(function *functionspb.CloudFunction, standard map[string]string) {
	if function.EnvironmentVariables == nil {
		function.EnvironmentVariables = make(map[string]string)
	}
	for key, val := range standard {
		function.EnvironmentVariables[key] = val
	}
}
