package compile

import (
	"strings"
	"unicode"
)

// toSnakeCase converts PascalCase to snake_case.
// e.g., "PluginVersion" -> "plugin_version", "ID" -> "id"
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	var words []string
	wordStart := 0

	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			if unicode.IsLower(runes[i-1]) {
				words = append(words, string(runes[wordStart:i]))
				wordStart = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				words = append(words, string(runes[wordStart:i]))
				wordStart = i
			}
		}
	}
	words = append(words, string(runes[wordStart:]))

	for i, w := range words {
		words[i] = strings.ToLower(w)
	}

	return strings.Join(words, "_")
}

// tableName converts a PascalCase model name to a pluralized snake_case table name.
// e.g., "Organization" -> "organizations", "PluginVersion" -> "plugin_versions"
func tableName(modelName string) string {
	return pluralize(toSnakeCase(modelName))
}

// pluralize adds a simple plural suffix.
func pluralize(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "z") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		prev := s[len(s)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	return s + "s"
}
