package compile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Organization", "organization"},
		{"PluginVersion", "plugin_version"},
		{"ID", "id"},
		{"FormatVersion", "format_version"},
		{"SpecificationVersion", "specification_version"},
		{"", ""},
		{"A", "a"},
		{"ABCDef", "abc_def"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := toSnakeCase(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTableName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Organization", "organizations"},
		{"PluginVersion", "plugin_versions"},
		{"Format", "formats"},
		{"Task", "tasks"},
		{"User", "users"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := tableName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"organization", "organizations"},
		{"format", "formats"},
		{"task", "tasks"},
		{"status", "statuses"},
		{"category", "categories"},
		{"key", "keys"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := pluralize(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
