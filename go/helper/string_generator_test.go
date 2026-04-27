package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomStringWithCharset(t *testing.T) {
	t.Run("should generate string of correct length", func(t *testing.T) {
		result := GenerateRandomStringWithCharset(10, "abc")
		assert.Len(t, result, 10)
	})

	t.Run("should only contain characters from charset", func(t *testing.T) {
		cs := "abc"
		result := GenerateRandomStringWithCharset(100, cs)
		for _, c := range result {
			assert.Contains(t, cs, string(c))
		}
	})

	t.Run("should generate empty string for length 0", func(t *testing.T) {
		result := GenerateRandomStringWithCharset(0, "abc")
		assert.Empty(t, result)
	})

	t.Run("should generate single char string for length 1", func(t *testing.T) {
		result := GenerateRandomStringWithCharset(1, "X")
		assert.Equal(t, "X", result)
	})
}

func TestGenerateRandomString(t *testing.T) {
	t.Run("should generate string of correct length", func(t *testing.T) {
		result := GenerateRandomString(20)
		assert.Len(t, result, 20)
	})

	t.Run("should generate different strings on multiple calls", func(t *testing.T) {
		r1 := GenerateRandomString(32)
		r2 := GenerateRandomString(32)
		// Extremely unlikely to be equal with 32 chars from 62-char charset
		assert.NotEqual(t, r1, r2)
	})

	t.Run("should only contain alphanumeric characters", func(t *testing.T) {
		result := GenerateRandomString(200)
		for _, c := range result {
			isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
			isDigit := c >= '0' && c <= '9'
			assert.True(t, isAlpha || isDigit, "unexpected character: %c", c)
		}
	})
}

func TestGenerateUUID(t *testing.T) {
	t.Run("should generate non-empty UUID", func(t *testing.T) {
		uuid := GenerateUUID()
		assert.NotEmpty(t, uuid)
	})

	t.Run("should generate unique UUIDs", func(t *testing.T) {
		uuid1 := GenerateUUID()
		uuid2 := GenerateUUID()
		assert.NotEqual(t, uuid1, uuid2)
	})

	t.Run("should have correct UUID format length", func(t *testing.T) {
		uuid := GenerateUUID()
		// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars)
		assert.Len(t, uuid, 36)
	})
}

func TestGenerateUUIDV4(t *testing.T) {
	t.Run("should generate non-empty UUID", func(t *testing.T) {
		uuid := GenerateUUIDV4()
		assert.NotEmpty(t, uuid)
	})

	t.Run("should generate unique UUIDs", func(t *testing.T) {
		uuid1 := GenerateUUIDV4()
		uuid2 := GenerateUUIDV4()
		assert.NotEqual(t, uuid1, uuid2)
	})

	t.Run("should have correct UUID format length", func(t *testing.T) {
		uuid := GenerateUUIDV4()
		assert.Len(t, uuid, 36)
	})
}
