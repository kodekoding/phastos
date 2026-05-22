package helper

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHexColor(t *testing.T) {
	t.Run("should parse 7-char hex color", func(t *testing.T) {
		c, err := ParseHexColor("#FF0000")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, c)
	})

	t.Run("should parse lowercase hex color", func(t *testing.T) {
		c, err := ParseHexColor("#00ff00")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 0, G: 255, B: 0, A: 255}, c)
	})

	t.Run("should parse blue hex color", func(t *testing.T) {
		c, err := ParseHexColor("#0000FF")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 255, A: 255}, c)
	})

	t.Run("should parse 4-char shorthand hex color", func(t *testing.T) {
		c, err := ParseHexColor("#F00")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, c)
	})

	t.Run("should parse white hex color", func(t *testing.T) {
		c, err := ParseHexColor("#FFFFFF")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, c)
	})

	t.Run("should parse black hex color", func(t *testing.T) {
		c, err := ParseHexColor("#000000")
		assert.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, c)
	})

	t.Run("should return error for invalid length", func(t *testing.T) {
		_, err := ParseHexColor("#FF00")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid length")
	})

	t.Run("should return error for empty string", func(t *testing.T) {
		_, err := ParseHexColor("")
		assert.Error(t, err)
	})

	t.Run("should return error for invalid hex chars", func(t *testing.T) {
		_, err := ParseHexColor("#GGGGGG")
		assert.Error(t, err)
	})
}

// ---- GetColorUniform tests ----

func TestGetColorUniform(t *testing.T) {
	t.Run("should return Uniform with valid 7-char hex color", func(t *testing.T) {
		uniform := GetColorUniform("#FF0000")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, uniform.C)
	})

	t.Run("should return Uniform with valid 4-char shorthand hex color", func(t *testing.T) {
		uniform := GetColorUniform("#F00")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, uniform.C)
	})

	t.Run("should return Uniform with lowercase hex color", func(t *testing.T) {
		uniform := GetColorUniform("#00ff00")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 0, G: 255, B: 0, A: 255}, uniform.C)
	})

	t.Run("should return Uniform with white hex color", func(t *testing.T) {
		uniform := GetColorUniform("#FFFFFF")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, uniform.C)
	})

	t.Run("should return Uniform with black hex color", func(t *testing.T) {
		uniform := GetColorUniform("#000000")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, uniform.C)
	})

	t.Run("should return zero-color Uniform for invalid hex color", func(t *testing.T) {
		uniform := GetColorUniform("#INVALID")
		assert.NotNil(t, uniform)
		// ParseHexColor sets A=0xff first, then only overwrites RGB on success.
		// On failure, A remains 0xff but RGB stays at 0.
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, uniform.C)
	})

	t.Run("should return zero-color Uniform for empty string", func(t *testing.T) {
		uniform := GetColorUniform("")
		assert.NotNil(t, uniform)
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, uniform.C)
	})
}
