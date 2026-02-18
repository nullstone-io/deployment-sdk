package operations

import (
	"context"
	"fmt"
	"strings"
	"sync"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

type GlobalGetter struct {
	TokenSource   oauth2.TokenSource
	OperationName string

	client *compute.GlobalOperationsClient
	once   sync.Once
}

func (g *GlobalGetter) Close() {
	if g.client != nil {
		g.client.Close()
	}
}

func (g *GlobalGetter) Get(ctx context.Context) (*computepb.Operation, error) {
	var err error
	g.once.Do(func() {
		g.client, err = compute.NewGlobalOperationsRESTClient(ctx, option.WithTokenSource(g.TokenSource))
	})
	if err != nil {
		return nil, fmt.Errorf("error creating google operations client: %w", err)
	}

	tokens := strings.Split(g.OperationName, "/")
	if len(tokens) < 5 {
		return nil, fmt.Errorf("invalid operation name %q", g.OperationName)
	}
	projectId := tokens[1]
	opId := tokens[4]

	req := &computepb.GetGlobalOperationRequest{
		Project:   projectId,
		Operation: opId,
	}
	return g.client.Get(ctx, req)
}
