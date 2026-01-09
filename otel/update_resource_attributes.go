package otel

func UpdateResourceAttributes(appVersion, commitSha string, isExpansionSupported bool) func(input string) string {
	return func(input string) string {
		result, err := ParseResourceAttributes(input)
		if err != nil {
			return input
		}

		// When expansion is supported, we won't overwrite a `$(...)` value
		if !isExpansionSupported || !result.IsExpansion("service.version") {
			result["service.version"] = appVersion
		}
		if !isExpansionSupported || !result.IsExpansion("service.commit.sha") {
			result["service.commit.sha"] = commitSha
		}
		return result.String()
	}
}
