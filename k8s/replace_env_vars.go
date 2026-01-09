package k8s

import (
	"github.com/nullstone-io/deployment-sdk/otel"
	core_v1 "k8s.io/api/core/v1"
)

func ReplaceEnvVars(container *core_v1.Container, std map[string]string) {
	for i, cur := range container.Env {
		if val, ok := std[cur.Name]; ok {
			ReplaceEnvVar(container, i, func(previous string) string { return val })
		}
	}
}

func ReplaceOtelResourceAttributesEnvVar(container *core_v1.Container, appVersion, commitSha string) bool {
	for i, cur := range container.Env {
		if cur.Name == otel.ResourceAttributesEnvName {
			ReplaceEnvVar(container, i, otel.UpdateResourceAttributes(appVersion, commitSha, true))
			return true
		}
	}
	return false
}

func ReplaceEnvVar(container *core_v1.Container, index int, fn func(previous string) string) {
	if ev := container.Env[index]; ev.ValueFrom == nil {
		ev.Value = fn(ev.Value)
		container.Env[index] = ev
	}
}
