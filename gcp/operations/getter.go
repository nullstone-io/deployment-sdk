package operations

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"golang.org/x/oauth2"
	"strings"
)

type Getter interface {
	Get(ctx context.Context) (*computepb.Operation, error)
	Close()
}

func NewGetter(tokenSource oauth2.TokenSource, operationName string) Getter {
	if strings.Contains(operationName, "/global/operations/") {
		return &GlobalGetter{
			TokenSource:   tokenSource,
			OperationName: operationName,
		}
	} else if strings.Contains(operationName, "/regions/") {
		return &RegionalGetter{
			TokenSource:   tokenSource,
			OperationName: operationName,
		}
	}
	return nil
}
