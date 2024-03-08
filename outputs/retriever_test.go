package outputs

import (
	"context"
	"github.com/google/uuid"
	"github.com/nullstone-io/module/config"
	"github.com/stretchr/testify/assert"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"testing"
)

type MockFlatOutputs struct {
	Output1 string            `ns:"output1"`
	Output2 int               `ns:"output2"`
	Output3 map[string]string `ns:"output3"`
}

type MockDeepOutputs struct {
	Output1 string          `ns:"output1"`
	Conn1   MockFlatOutputs `ns:",connectionType:aws-flat"`
	Conn2   MockFlatOutputs `ns:",connectionContract:app/aws/flat"`
}

func TestRetriever_Retrieve(t *testing.T) {
	flatWorkspace := &types.Workspace{
		UidCreatedModel: types.UidCreatedModel{Uid: uuid.New()},
		OrgName:         "default",
		StackId:         1,
		BlockId:         5,
		EnvId:           15,
	}
	flatWorkspaceOutputs := types.Outputs{
		"output1": types.Output{
			Type:      "string",
			Value:     "value1",
			Sensitive: false,
		},
		"output2": types.Output{
			Type:      "number",
			Value:     2,
			Sensitive: false,
		},
		"output3": types.Output{
			Type: "map(string)",
			Value: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			Sensitive: false,
		},
	}
	flat2Workspace := &types.Workspace{
		UidCreatedModel: types.UidCreatedModel{Uid: uuid.New()},
		OrgName:         "default",
		StackId:         1,
		BlockId:         7,
		EnvId:           15,
	}
	flat2WorkspaceOutputs := types.Outputs{
		"output1": types.Output{
			Type:      "string",
			Value:     "value1",
			Sensitive: false,
		},
		"output2": types.Output{
			Type:      "number",
			Value:     2,
			Sensitive: false,
		},
		"output3": types.Output{
			Type: "map(string)",
			Value: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			Sensitive: false,
		},
	}
	deepWorkspace := &types.Workspace{
		UidCreatedModel: types.UidCreatedModel{Uid: uuid.New()},
		OrgName:         "default",
		StackId:         1,
		BlockId:         6,
		EnvId:           15,
		LastFinishedRun: &types.Run{
			Config: &types.RunConfig{
				Connections: map[string]types.Connection{
					"deep": {
						Connection: config.Connection{
							Type:     "aws-flat",
							Optional: false,
						},
						Target: "deep0",
						Reference: &types.ConnectionTarget{
							StackId: 1,
							BlockId: 5,
							EnvId:   nil,
						},
						Unused: false,
					},
					"deep2": {
						Connection: config.Connection{
							Contract: "app/aws/flat",
							Optional: false,
						},
						Target: "deep2",
						Reference: &types.ConnectionTarget{
							StackId: 1,
							BlockId: 7,
							EnvId:   nil,
						},
						Unused: false,
					},
				},
			},
		},
	}
	deepWorkspaceOutputs := types.Outputs{
		"output1": types.Output{
			Type:      "string",
			Value:     "test",
			Sensitive: false,
		},
	}
	allWorkspaces := []types.Workspace{
		*deepWorkspace,
		*flat2Workspace,
		*flatWorkspace,
	}
	allOutputs := map[string]types.Outputs{
		flatWorkspace.Uid.String():  flatWorkspaceOutputs,
		flat2Workspace.Uid.String(): flat2WorkspaceOutputs,
		deepWorkspace.Uid.String():  deepWorkspaceOutputs,
	}

	t.Run("should retrieve outputs for single workspace", func(t *testing.T) {
		server, nsConfig := mockNs([]types.Workspace{*flatWorkspace}, map[string]types.Outputs{flatWorkspace.Uid.String(): flatWorkspaceOutputs})
		t.Cleanup(server.Close)

		want := MockFlatOutputs{
			Output1: "value1",
			Output2: 2,
			Output3: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		}

		retriever := Retriever{Source: ApiRetrieverSource{Config: nsConfig}}
		var got MockFlatOutputs
		if assert.NoError(t, retriever.Retrieve(context.Background(), flatWorkspace, &got)) {
			assert.Equal(t, want, got)
		}
	})

	t.Run("should retrieve outputs for own workspace and connected workspaces", func(t *testing.T) {
		server, nsConfig := mockNs(allWorkspaces, allOutputs)
		t.Cleanup(server.Close)

		want := MockDeepOutputs{
			Output1: "test",
			Conn1: MockFlatOutputs{
				Output1: "value1",
				Output2: 2,
				Output3: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			Conn2: MockFlatOutputs{
				Output1: "value1",
				Output2: 2,
				Output3: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
		}

		retriever := Retriever{Source: ApiRetrieverSource{Config: nsConfig}}
		var got MockDeepOutputs
		if assert.NoError(t, retriever.Retrieve(context.Background(), deepWorkspace, &got)) {
			assert.Equal(t, want, got)
		}
	})
}
