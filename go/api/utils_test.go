package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestArrContains(t *testing.T) {
	t.Run("string slice - found", func(t *testing.T) {
		assert.True(t, ArrContains([]string{"a", "b", "c"}, "b"))
	})

	t.Run("string slice - not found", func(t *testing.T) {
		assert.False(t, ArrContains([]string{"a", "b", "c"}, "d"))
	})

	t.Run("int slice - found", func(t *testing.T) {
		assert.True(t, ArrContains([]int{1, 2, 3}, 2))
	})

	t.Run("int slice - not found", func(t *testing.T) {
		assert.False(t, ArrContains([]int{1, 2, 3}, 4))
	})

	t.Run("float32 slice - found", func(t *testing.T) {
		assert.True(t, ArrContains([]float32{1.1, 2.2, 3.3}, float32(2.2)))
	})

	t.Run("empty slice", func(t *testing.T) {
		assert.False(t, ArrContains([]string{}, "a"))
	})
}

func TestGetStructKey(t *testing.T) {
	type testStruct struct {
		WithDB    string `db:"db_field" json:"json_field"`
		OnlyJSON  string `json:"json_only"`
		NoTags    string
		SkipField string `db:"-"`
	}

	typ := reflect.TypeOf(testStruct{})

	t.Run("should prefer db tag", func(t *testing.T) {
		field, _ := typ.FieldByName("WithDB")
		assert.Equal(t, "db_field", GetStructKey(field))
	})

	t.Run("should fallback to json tag", func(t *testing.T) {
		field, _ := typ.FieldByName("OnlyJSON")
		assert.Equal(t, "json_only", GetStructKey(field))
	})

	t.Run("should fallback to lowercase field name", func(t *testing.T) {
		field, _ := typ.FieldByName("NoTags")
		assert.Equal(t, "notags", GetStructKey(field))
	})
}

func TestGetStructValue(t *testing.T) {
	t.Run("should handle int", func(t *testing.T) {
		val := reflect.ValueOf(42)
		result := GetStructValue(val)
		assert.Equal(t, int64(42), result)
	})

	t.Run("should handle float64", func(t *testing.T) {
		val := reflect.ValueOf(3.14)
		result := GetStructValue(val)
		assert.Equal(t, 3.14, result)
	})

	t.Run("should handle bool", func(t *testing.T) {
		val := reflect.ValueOf(true)
		result := GetStructValue(val)
		assert.Equal(t, true, result)
	})

	t.Run("should handle string", func(t *testing.T) {
		val := reflect.ValueOf("hello")
		result := GetStructValue(val)
		assert.Equal(t, "hello", result)
	})

	t.Run("should handle time.Time", func(t *testing.T) {
		now := time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC)
		val := reflect.ValueOf(now)
		result := GetStructValue(val)
		assert.Contains(t, result, "2026-05-15")
	})
}

func TestValidateStruct(t *testing.T) {
	type testData struct {
		Name  string `validate:"required"`
		Email string `validate:"required"`
	}

	t.Run("should return no errors for valid struct", func(t *testing.T) {
		data := testData{Name: "John", Email: "john@example.com"}
		errors := ValidateStruct(data)
		assert.Empty(t, errors)
	})

	t.Run("should return errors for invalid struct", func(t *testing.T) {
		data := testData{Name: "", Email: ""}
		errors := ValidateStruct(data)
		assert.Len(t, errors, 2)
	})

	t.Run("should return error for partially invalid struct", func(t *testing.T) {
		data := testData{Name: "John", Email: ""}
		errors := ValidateStruct(data)
		assert.Len(t, errors, 1)
		assert.Equal(t, "required", errors[0].Tag)
	})
}

func TestFilterFlags(t *testing.T) {
	t.Run("should return content without flags", func(t *testing.T) {
		assert.Equal(t, "application/json", filterFlags("application/json"))
	})

	t.Run("should strip after semicolon", func(t *testing.T) {
		assert.Equal(t, "application/json", filterFlags("application/json; charset=utf-8"))
	})

	t.Run("should strip after space", func(t *testing.T) {
		assert.Equal(t, "text/html", filterFlags("text/html charset=utf-8"))
	})

	t.Run("should handle empty string", func(t *testing.T) {
		assert.Equal(t, "", filterFlags(""))
	})
}

func TestGenerateQueryComponenFromStruct(t *testing.T) {
	type testModel struct {
		Name  string `db:"name" json:"name"`
		Email string `db:"email" json:"email"`
		Age   int    `db:"age" json:"age"`
	}

	t.Run("should generate fields, values, and binds", func(t *testing.T) {
		model := testModel{Name: "John", Email: "john@test.com", Age: 30}
		fields, values, binds := GenerateQueryComponenFromStruct(model, []string{})

		assert.Equal(t, "name, email, age", fields)
		assert.Len(t, values, 3)
		assert.Equal(t, "$1, $2, $3", binds)
	})

	t.Run("should skip specified fields", func(t *testing.T) {
		model := testModel{Name: "John", Email: "john@test.com", Age: 30}
		fields, values, binds := GenerateQueryComponenFromStruct(model, []string{"name"})

		// "name" is the first field, so it stops there
		assert.Equal(t, "", fields)
		assert.Empty(t, values)
		assert.Equal(t, "", binds)
	})
}
