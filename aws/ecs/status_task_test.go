package ecs

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestStatusTaskContainerFromEcs_CopiesImage(t *testing.T) {
	c := StatusTaskContainerFromEcs(ecstypes.Container{
		Name:  aws.String("main"),
		Image: aws.String("123.dkr.ecr.us-east-1.amazonaws.com/app:rev5"),
	})
	if c.Image != "123.dkr.ecr.us-east-1.amazonaws.com/app:rev5" {
		t.Errorf("image = %q", c.Image)
	}
}

// A historical execution ran an older revision; the current definition must not relabel it.
func TestStatusTaskContainer_Enrich_KeepsReportedImage(t *testing.T) {
	c := StatusTaskContainer{Name: "main", Image: "app:rev5"}
	taskDef := &ecstypes.TaskDefinition{ContainerDefinitions: []ecstypes.ContainerDefinition{
		{Name: aws.String("main"), Image: aws.String("app:rev7")},
	}}
	c.Enrich(nil, taskDef)
	if c.Image != "app:rev5" {
		t.Errorf("image = %q, want app:rev5", c.Image)
	}
}

func TestStatusTaskContainer_Enrich_FallsBackToDefinitionImage(t *testing.T) {
	c := StatusTaskContainer{Name: "main"}
	taskDef := &ecstypes.TaskDefinition{ContainerDefinitions: []ecstypes.ContainerDefinition{
		{Name: aws.String("main"), Image: aws.String("app:rev7")},
	}}
	c.Enrich(nil, taskDef)
	if c.Image != "app:rev7" {
		t.Errorf("image = %q, want app:rev7", c.Image)
	}
	c = StatusTaskContainer{Name: "main"}
	c.Enrich(nil, nil)
	if c.Image != "" {
		t.Errorf("image = %q, want empty without a definition", c.Image)
	}
}
