package otel

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ResourceAttributesEnvName = "OTEL_RESOURCE_ATTRIBUTES"
)

func ParseResourceAttributes(input string) (ResourceAttributes, error) {
	attrs := ResourceAttributes{}
	if strings.TrimSpace(input) == "" {
		return attrs, nil
	}

	pairs := splitPairs(input)

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		key, val, ok := splitPair(pair)
		if !ok {
			return nil, fmt.Errorf("invalid OTEL_RESOURCE_ATTRIBUTES entry: %s", pair)
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if key == "" {
			return nil, fmt.Errorf("empty attribute key")
		}

		attrs[key] = val
	}

	return attrs, nil
}

type ResourceAttributes map[string]string

func (a ResourceAttributes) String() string {
	if len(a) == 0 {
		return ""
	}

	keys := make([]string, 0, len(a))
	for k := range a {
		if strings.TrimSpace(k) == "" {
			continue
		}
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}

		b.WriteString(escapeOTEL(k))
		b.WriteByte('=')
		b.WriteString(escapeOTEL(a[k]))
	}

	return b.String()
}

// escapeOTEL escapes characters required by the OTEL env var spec.
func escapeOTEL(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', ',', '=':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitPairs splits input on unescaped commas, preserving escaped commas within values
func splitPairs(input string) []string {
	var parts []string
	var current strings.Builder

	for i, r := range input {
		isEscaped := i > 0 && input[i-1] == '\\'
		if r == ',' && !isEscaped {
			// Only split on unescaped commas
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(r)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// splitPair splits input on the first unescaped '=' rune
func splitPair(input string) (string, string, bool) {
	var key strings.Builder

	escaped := false
	for i, r := range input {
		switch {
		case escaped:
			key.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '=':
			// Found the first unescaped '='
			return key.String(), unescape(input[i+1:]), true
		default:
			key.WriteRune(r)
		}
	}

	// No unescaped '=' found
	return "", "", false
}

func unescape(input string) string {
	var current strings.Builder
	escaped := false
	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		default:
			current.WriteRune(r)
		}
	}
	return current.String()
}
