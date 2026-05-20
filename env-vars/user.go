package env_vars

import (
	"fmt"
	"regexp"

	"github.com/nullstone-io/deployment-sdk/app"
)

// interpolationRefRegexPattern matches a handlebar reference to an env var, e.g. `{{ NAME }}`.
// It mirrors the syntax used by terraform-provider-ns (internal/provider/env_vars.go).
const interpolationRefRegexPattern = "{{\\s*%s\\s*}}"

// maxInterpolationPasses caps convergence looping to guard against pathological/cyclic references.
const maxInterpolationPasses = 100

// ResolveUser returns the user-supplied env vars (meta.EnvVars) with interpolation applied.
//
// Interpolation uses handlebar syntax ({{ VAR }}) and only resolves references against:
//   - other user-supplied env vars (meta.EnvVars)
//   - the standard/built-in env vars (NULLSTONE_VERSION, NULLSTONE_COMMIT_SHA)
//
// It deliberately does not resolve against env vars already present on the infra resource,
// which means secret-backed env vars can never be referenced (and therefore never leaked).
// A reference to an unknown var is left untouched (as a literal {{ VAR }}).
func ResolveUser(meta app.DeployMetadata) map[string]string {
	if len(meta.EnvVars) == 0 {
		return map[string]string{}
	}

	// resolved holds the user values we interpolate into and ultimately return.
	resolved := make(map[string]string, len(meta.EnvVars))
	for k, v := range meta.EnvVars {
		resolved[k] = v
	}

	// context is the lookup source: standard env vars overlaid by user values (user wins).
	context := map[string]string{}
	for k, v := range GetStandard(meta) {
		context[k] = v
	}
	for k, v := range resolved {
		context[k] = v
	}

	// Replace {{ NAME }} references with the context value, looping until convergence to handle
	// chained references (e.g. A -> B -> NULLSTONE_VERSION). A var is never substituted into
	// itself, which keeps cyclic references from looping forever (they are left as literals).
	for changed, pass := true, 0; changed && pass < maxInterpolationPasses; pass++ {
		changed = false
		for refName, refValue := range context {
			replacer := regexp.MustCompile(fmt.Sprintf(interpolationRefRegexPattern, regexp.QuoteMeta(refName)))
			for name, value := range resolved {
				if name == refName {
					continue
				}
				result := replacer.ReplaceAllLiteralString(value, refValue)
				if result != value {
					changed = true
					resolved[name] = result
					context[name] = result
				}
			}
		}
	}

	return resolved
}
