package workspace

import (
	"context"

	"github.com/nullstone-io/deployment-sdk/contract"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type NewActionerFunc func(ctx context.Context, source outputs.RetrieverSource, blockDetails Details) (Actioner, error)

type Actioners map[types.ModuleContractName]NewActionerFunc

func (s Actioners) FindActioner(ctx context.Context, source outputs.RetrieverSource, blockDetails Details) (Actioner, error) {
	fn := contract.FindInRegistrarByModule(s, blockDetails.Module)
	if fn == nil || *fn == nil {
		return nil, nil
	}
	a, err := (*fn)(ctx, source, blockDetails)
	if err != nil {
		return nil, ActionNotSupportedError{InnerErr: err}
	}
	return a, nil
}
