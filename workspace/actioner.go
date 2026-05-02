package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type Actioner interface {
	PerformAction(ctx context.Context, options ActionOptions) (*ActionResult, error)
}

type ActionOptions struct {
	// Action identifies the operation to perform (e.g. "restart-deployment", "rerun-job", "kill-pod").
	// Each Actioner implementation defines the set of actions it supports.
	Action string

	// Input is the action-specific request payload.
	// Each implementation unmarshals this into its own typed struct.
	Input json.RawMessage
}

type ActionResult struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func IsActionNotSupported(err error) (ActionNotSupportedError, bool) {
	var anse ActionNotSupportedError
	if errors.As(err, &anse) {
		return anse, true
	}
	return anse, false
}

var _ error = ActionNotSupportedError{}

type ActionNotSupportedError struct {
	InnerErr error
}

func (e ActionNotSupportedError) Error() string {
	return fmt.Sprintf("action not supported: %s", e.InnerErr)
}

func (e ActionNotSupportedError) Unwrap() error {
	return e.InnerErr
}
