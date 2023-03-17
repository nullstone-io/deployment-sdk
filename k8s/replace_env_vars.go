package k8s

import (
	v1 "k8s.io/api/core/v1"
)

func ReplaceEnvVars(podSpec v1.PodSpec, std map[string]string) v1.PodSpec {
	updateEnv := func(env []v1.EnvVar) []v1.EnvVar {
		updated := make([]v1.EnvVar, 0)
		for _, cur := range env {
			if val, ok := std[cur.Name]; ok && cur.ValueFrom == nil {
				cur.Value = val
			}
			updated = append(updated, cur)
		}
		return updated
	}

	for i, cd := range podSpec.Containers {
		cd.Env = updateEnv(cd.Env)
		podSpec.Containers[i] = cd
	}

	return podSpec
}
