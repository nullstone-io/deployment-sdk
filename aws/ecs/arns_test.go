package ecs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseClusterName(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{"standard arn", "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster", "my-cluster"},
		{"cluster name with hyphens", "arn:aws:ecs:us-west-2:123456789012:cluster/prod-app-cluster", "prod-app-cluster"},
		{"empty arn", "", ""},
		{"malformed arn (no slash)", "arn:aws:ecs:us-east-1:123456789012:cluster", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseClusterName(tt.arn))
		})
	}
}

func TestParseTaskDefinition(t *testing.T) {
	tests := []struct {
		name        string
		arn         string
		wantFamily  string
		wantRevision int32
	}{
		{"standard arn", "arn:aws:ecs:us-east-1:123456789012:task-definition/my-app:42", "my-app", 42},
		{"family with hyphens", "arn:aws:ecs:us-east-1:123456789012:task-definition/prod-api-svc:7", "prod-api-svc", 7},
		{"large revision", "arn:aws:ecs:us-east-1:123456789012:task-definition/my-app:1234567", "my-app", 1234567},
		{"empty arn", "", "", 0},
		{"missing revision", "arn:aws:ecs:us-east-1:123456789012:task-definition/my-app", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family, rev := parseTaskDefinition(tt.arn)
			assert.Equal(t, tt.wantFamily, family)
			assert.Equal(t, tt.wantRevision, rev)
		})
	}
}
