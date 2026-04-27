package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceContains(t *testing.T) {
	t.Run("should return true when string exists in slice", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		assert.True(t, SliceContains(slice, "banana"))
	})

	t.Run("should return false when string does not exist in slice", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		assert.False(t, SliceContains(slice, "grape"))
	})

	t.Run("should return false for empty slice", func(t *testing.T) {
		var slice []string
		assert.False(t, SliceContains(slice, "anything"))
	})

	t.Run("should return true for first element", func(t *testing.T) {
		slice := []string{"first", "second", "third"}
		assert.True(t, SliceContains(slice, "first"))
	})

	t.Run("should return true for last element", func(t *testing.T) {
		slice := []string{"first", "second", "third"}
		assert.True(t, SliceContains(slice, "third"))
	})

	t.Run("should be case sensitive", func(t *testing.T) {
		slice := []string{"Apple", "Banana"}
		assert.False(t, SliceContains(slice, "apple"))
		assert.True(t, SliceContains(slice, "Apple"))
	})

	t.Run("should handle empty string in slice", func(t *testing.T) {
		slice := []string{"", "hello"}
		assert.True(t, SliceContains(slice, ""))
	})
}

func TestRemove(t *testing.T) {
	t.Run("should remove element at given index", func(t *testing.T) {
		slice := []interface{}{"a", "b", "c", "d"}
		result := Remove(slice, 1)
		assert.Equal(t, []interface{}{"a", "c", "d"}, result)
	})

	t.Run("should remove first element", func(t *testing.T) {
		slice := []interface{}{1, 2, 3}
		result := Remove(slice, 0)
		assert.Equal(t, []interface{}{2, 3}, result)
	})

	t.Run("should remove last element", func(t *testing.T) {
		slice := []interface{}{1, 2, 3}
		result := Remove(slice, 2)
		assert.Equal(t, []interface{}{1, 2}, result)
	})
}
