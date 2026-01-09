package otel

func UpdateResourceAttributes(appVersion, commitSha string) func(input string) string {
	return func(input string) string {
		result, err := ParseResourceAttributes(input)
		if err != nil {
			return input
		}

		result["service.version"] = appVersion
		result["service.commit.sha"] = commitSha
		return result.String()
	}
}
