package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseToDateOnly(t *testing.T) {
	t.Run("should parse RFC3339 to date only", func(t *testing.T) {
		result := ParseToDateOnly("2026-05-15T14:30:00Z")
		assert.Equal(t, "2026-05-15", result)
	})

	t.Run("should parse RFC3339 with timezone offset", func(t *testing.T) {
		result := ParseToDateOnly("2026-12-25T08:00:00+07:00")
		assert.Equal(t, "2026-12-25", result)
	})

	t.Run("should return zero date for invalid input", func(t *testing.T) {
		result := ParseToDateOnly("not-a-date")
		assert.Equal(t, "0001-01-01", result)
	})
}

func TestParseToTimeOnly(t *testing.T) {
	t.Run("should parse RFC3339 to time only", func(t *testing.T) {
		result := ParseToTimeOnly("2026-05-15T14:30:45Z")
		assert.Equal(t, "14:30:45", result)
	})

	t.Run("should return zero time for invalid input", func(t *testing.T) {
		result := ParseToTimeOnly("not-a-time")
		assert.Equal(t, "00:00:00", result)
	})
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{"YYYY-MM-DD format", "2026-05-15", "2026-05-15", false},
		{"DD-MM-YYYY format", "15-05-2026", "2026-05-15", false},
		{"DD/MM/YYYY format", "15/05/2026", "2026-05-15", false},
		{"MM/DD/YYYY format", "05/15/2026", "2026-05-15", false},
		{"Month DD, YYYY format", "May 15, 2026", "2026-05-15", false},
		{"DD Mon YYYY format", "15 May 2026", "2026-05-15", false},
		{"YYYY.MM.DD format", "2026.05.15", "2026-05-15", false},
		{"invalid format", "not-a-date", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unable to parse date")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
