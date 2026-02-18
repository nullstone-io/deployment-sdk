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

type RegionalGetter struct {
	TokenSource   oauth2.TokenSource
	OperationName string

	client *compute.RegionOperationsClient
	once   sync.Once
}

func (g *RegionalGetter) Close() {
	if g.client != nil {
		g.client.Close()
	}
}

func (g *RegionalGetter) Get(ctx context.Context) (*computepb.Operation, error) {
	var err error
	g.once.Do(func() {
		g.client, err = compute.NewRegionOperationsRESTClient(ctx, option.WithTokenSource(g.TokenSource))
	})
	if err != nil {
		return nil, fmt.Errorf("error creating google operations client: %w", err)
	}

	tokens := strings.Split(g.OperationName, "/")
	if len(tokens) < 6 {
		return nil, fmt.Errorf("invalid operation name %q", g.OperationName)
	}
	projectId := tokens[1]
	region := tokens[3]
	opId := tokens[5]

	req := &computepb.GetRegionOperationRequest{
		Project:   projectId,
		Region:    region,
		Operation: opId,
	}
	return g.client.Get(ctx, req)
}
