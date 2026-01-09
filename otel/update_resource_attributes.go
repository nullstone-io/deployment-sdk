package otel

func UpdateResourceAttributes(appVersion, commitSha string, skipExpansion bool) func(input string) string {
	return func(input string) string {
		result, err := ParseResourceAttributes(input)
		if err != nil {
			return input
		}

		// When Skip Expansion is enabled, we won't overwrite a `$(...)` value
		if !skipExpansion || !result.IsExpansion("service.version") {
			result["service.version"] = appVersion
		}
		if !skipExpansion || !result.IsExpansion("service.commit.sha") {
			result["service.commit.sha"] = commitSha
		}
		return result.String()
	}
}
