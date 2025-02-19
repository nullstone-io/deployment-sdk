package outputs

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"reflect"
)

type RetrieverSource interface {
	GetWorkspace(ctx context.Context, stackId, blockId, envId int64) (*types.Workspace, error)
	GetCurrentConfig(ctx context.Context, stackId, blockId, envId int64) (*types.WorkspaceConfig, error)
	GetCurrentOutputs(ctx context.Context, stackId int64, workspaceUid uuid.UUID, showSensitive bool) (types.Outputs, error)
	GetTemporaryCredentials(ctx context.Context, stackId int64, workspaceUid uuid.UUID, input api.GenerateCredentialsInput) (*types.OutputCredentials, error)
}

func NewRetrieveWorkspace(workspace *types.Workspace, workspaceConfig *types.WorkspaceConfig) *RetrieveWorkspace {
	if workspace == nil {
		return nil
	}
	var wc types.WorkspaceConfig
	if workspaceConfig != nil {
		wc = *workspaceConfig
	} else if workspace.LastFinishedRun != nil && workspace.LastFinishedRun.Config != nil {
		lfr := workspace.LastFinishedRun.Config
		wc = types.WorkspaceConfig{
			Source:            lfr.Source,
			SourceVersion:     lfr.SourceVersion,
			Variables:         lfr.Variables,
			EnvVariables:      lfr.EnvVariables,
			Connections:       lfr.Connections,
			Providers:         lfr.Providers,
			Capabilities:      lfr.Capabilities,
			Dependencies:      lfr.Dependencies,
			DependencyConfigs: lfr.DependencyConfigs,
		}
	}

	return &RetrieveWorkspace{
		OrgName:      workspace.OrgName,
		WorkspaceUid: workspace.Uid,
		StackId:      workspace.StackId,
		BlockId:      workspace.BlockId,
		EnvId:        workspace.EnvId,
		Config:       wc,
	}
}

type RetrieveWorkspace struct {
	OrgName      string                `json:"orgName"`
	WorkspaceUid uuid.UUID             `json:"workspaceUid"`
	StackId      int64                 `json:"stackId"`
	BlockId      int64                 `json:"blockId"`
	EnvId        int64                 `json:"envId"`
	Config       types.WorkspaceConfig `json:"config"`
}

func (w RetrieveWorkspace) Id() string {
	return fmt.Sprintf("%s/%s", w.OrgName, w.WorkspaceUid)
}

func Retrieve[T any](ctx context.Context, source RetrieverSource, workspace *types.Workspace, workspaceConfig *types.WorkspaceConfig) (T, error) {
	rw := NewRetrieveWorkspace(workspace, workspaceConfig)
	var t T
	r := Retriever{Source: source}
	if err := r.Retrieve(ctx, rw, &t); err != nil {
		return t, err
	}
	return t, nil
}

type Retriever struct {
	Source RetrieverSource
}

var _ error = NoWorkspaceOutputsError{}

type NoWorkspaceOutputsError struct {
	workspace RetrieveWorkspace
}

func (n NoWorkspaceOutputsError) Error() string {
	return fmt.Sprintf("No outputs found for workspace %s/%d/%d/%d", n.workspace.OrgName, n.workspace.StackId, n.workspace.BlockId, n.workspace.EnvId)
}

// Retrieve is capable of retrieving all outputs for a given workspace
// To properly use, the input obj must be a pointer to a struct that contains fields that map to outputs
// Struct tags on each field within the struct define how to read the outputs from nullstone APIs
// See Field for more details
func (r *Retriever) Retrieve(ctx context.Context, rw *RetrieveWorkspace, obj interface{}) error {
	objType := reflect.TypeOf(obj)
	if objType.Kind() != reflect.Ptr {
		return fmt.Errorf("input object must be a pointer")
	}
	if objType.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("input object must be a pointer to a struct")
	}

	workspaceOutputs, err := r.Source.GetCurrentOutputs(ctx, rw.StackId, rw.WorkspaceUid, true)
	if err != nil {
		return fmt.Errorf("unable to fetch the outputs for %s: %w", rw.Id(), err)
	}
	if len(workspaceOutputs) == 0 {
		return NoWorkspaceOutputsError{workspace: *rw}
	}

	fields := GetFields(reflect.TypeOf(obj).Elem())
	for _, field := range fields {
		fieldType := field.Field.Type

		if field.Name == "" {
			// `ns:",..."` refers to a connection, this field must be a struct type
			//we're going to run retrieve into this field
			if err := CheckValidConnectionField(obj, fieldType); err != nil {
				return err
			}
			target := field.InitializeConnectionValue(obj)

			connWorkspace, err := r.GetConnectionWorkspace(ctx, rw, field.ConnectionName, field.ConnectionType, field.ConnectionContract)
			if err != nil {
				return fmt.Errorf("error finding connection workspace (name=%s, type=%s, contract=%s): %w", field.ConnectionName, field.ConnectionType, field.ConnectionContract, err)
			}
			if connWorkspace == nil {
				if field.Optional {
					continue
				}
				return ErrMissingRequiredConnection{
					ConnectionName:     field.ConnectionName,
					ConnectionType:     field.ConnectionType,
					ConnectionContract: field.ConnectionContract,
				}
			}
			if err := r.Retrieve(ctx, connWorkspace, target); err != nil {
				return err
			}
		} else {
			// `ns:"xyz"` refers to an output named `xyz` in the current workspace outputs
			// we're going to extract the value into this field
			if err := CheckValidField(obj, fieldType); err != nil {
				return err
			}
			if err := field.SafeSet(obj, workspaceOutputs); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetConnectionWorkspace gets the workspace from nullstone through a connection from the source workspace
// This will search through connections matching on connectionName and connectionType
// Specify "" to ignore filtering for that field
// One of either connectionName or connectionType must be specified
func (r *Retriever) GetConnectionWorkspace(ctx context.Context, source *RetrieveWorkspace, connectionName, connectionType, connectionContract string) (*RetrieveWorkspace, error) {
	conn, err := findConnection(source, connectionName, connectionType, connectionContract)
	if err != nil {
		return nil, err
	} else if conn == nil || conn.EffectiveTarget == nil {
		return nil, nil
	}

	sourceTarget := types.WorkspaceTarget{
		StackId: source.StackId,
		BlockId: source.BlockId,
		EnvId:   source.EnvId,
	}
	destTarget := sourceTarget.FindRelativeConnection(*conn.EffectiveTarget)

	workspace, err := r.Source.GetWorkspace(ctx, destTarget.StackId, destTarget.BlockId, destTarget.EnvId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspace: %w", err)
	} else if workspace == nil {
		return nil, nil
	}

	workspaceConfig, err := r.Source.GetCurrentConfig(ctx, destTarget.StackId, destTarget.BlockId, destTarget.EnvId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving current workspace config: %w", err)
	} else if workspaceConfig == nil {
		return nil, nil
	}
	return &RetrieveWorkspace{
		OrgName:      source.OrgName,
		WorkspaceUid: workspace.Uid,
		StackId:      destTarget.StackId,
		BlockId:      destTarget.BlockId,
		EnvId:        destTarget.EnvId,
		Config:       *workspaceConfig,
	}, nil
}

func findConnection(source *RetrieveWorkspace, connectionName, connectionType, connectionContract string) (*types.Connection, error) {
	if source == nil {
		return nil, fmt.Errorf("cannot find connections for app, workspace does not have a configuration")
	}
	hasType := connectionType != ""
	hasContract := connectionContract != ""
	if connectionName == "" && (!hasType && !hasContract) {
		return nil, fmt.Errorf("cannot find connection if name or type/contract is not specified")
	}
	var desiredContract types.ModuleContractName
	if hasContract {
		var err error
		if desiredContract, err = types.ParseModuleContractName(connectionContract); err != nil {
			return nil, fmt.Errorf("invalid connection contract %q: %w", connectionContract, err)
		}
	}

	for name, connection := range source.Config.Connections {
		curConnContract, err := types.ParseModuleContractName(connection.Contract)
		if err != nil {
			// We are skipping connections with bad contracts in the current run config
			continue
		}
		if hasContract && !desiredContract.Match(curConnContract) {
			continue
		}
		if hasType && connectionType != connection.Type {
			continue
		}
		if connectionName != "" && connectionName != name {
			continue
		}
		return &connection, nil
	}
	return nil, nil
}
