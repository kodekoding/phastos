package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"snake_case to CamelCase", "any_kind_of_string", "AnyKindOfString"},
		{"space separated", "any kind of string", "AnyKindOfString"},
		{"mixed format", "AnyKind of_string", "AnyKindOfString"},
		{"single word", "hello", "Hello"},
		{"already CamelCase", "HelloWorld", "HelloWorld"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCamelCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToLowerCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"snake_case to lowerCamelCase", "any_kind_of_string", "anyKindOfString"},
		{"space separated", "any kind of string", "anyKindOfString"},
		{"single word", "hello", "hello"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLowerCamelCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"CamelCase to snake_case", "AnyKindOfString", "any_kind_of_string"},
		{"lowerCamelCase", "anyKindOfString", "any_kind_of_string"},
		{"single word", "hello", "hello"},
		{"already snake_case", "already_snake", "already_snake"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrimDuplicatedSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"multiple spaces", "hello   world", "hello world"},
		{"tabs and spaces", "hello\t\tworld", "hello world"},
		{"leading and trailing spaces", "  hello  world  ", " hello world "},
		{"single space", "hello world", "hello world"},
		{"no spaces", "helloworld", "helloworld"},
		{"newlines", "hello\n\nworld", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimDuplicatedSpace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
