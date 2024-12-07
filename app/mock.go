package app

import (
	"context"
	"github.com/stretchr/testify/mock"
)

var _ DeployStatusGetter = &MockDeployStatusGetter{}

type MockDeployStatusGetter struct {
	mock.Mock
}

func (m *MockDeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (RolloutStatus, error) {
	args := m.MethodCalled("GetDeployStatus", ctx, reference)
	return args.Get(0).(RolloutStatus), args.Error(1)
}

func (m *MockDeployStatusGetter) Close() {
	m.MethodCalled("Close")
}
