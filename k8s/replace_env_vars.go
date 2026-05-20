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

// ApplyUserEnvVars upserts user-supplied (deploy-time) env vars into the container.
// Unlike ReplaceEnvVars, this adds env vars that don't already exist in addition to overriding existing ones.
// Existing entries backed by ValueFrom (e.g. secrets) are converted to a literal value.
func ApplyUserEnvVars(container *core_v1.Container, userEnvVars map[string]string) bool {
	if len(userEnvVars) == 0 {
		return false
	}

	for name, value := range userEnvVars {
		upsertEnvVar(container, name, value)
	}
	return true
}

func upsertEnvVar(container *core_v1.Container, name, value string) {
	for i, cur := range container.Env {
		if cur.Name == name {
			container.Env[i] = core_v1.EnvVar{Name: name, Value: value}
			return
		}
	}
	container.Env = append(container.Env, core_v1.EnvVar{Name: name, Value: value})
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
