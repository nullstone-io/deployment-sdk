package k8s

import (
	core_v1 "k8s.io/api/core/v1"
)

func ReplaceEnvVars(container *core_v1.Container, std map[string]string) {
	for k, cur := range container.Env {
		if val, ok := std[cur.Name]; ok && cur.ValueFrom == nil {
			container.Env[k].Value = val
		}
	}
}
