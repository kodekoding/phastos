package binding

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/volatiletech/null"
)

func Test_nullValidator(t *testing.T) {

	t.Run("Should return error field is not numeric", func(t *testing.T) {
		type mockInnerStruct struct {
			StringGt2NotNumeric null.String `db:"StringGt2NotNumeric" json:"StringGt2NotNumeric" schema:"StringGt2NotNumeric" validate:"gt=2"`
		}
		mockParam := struct {
			InnerStruct     mockInnerStruct
			SkipStringField string `db:"skip_string_field" json:"skip_string_field" schema:"skip_string_field" validate:"required"`
		}{
			InnerStruct: mockInnerStruct{
				StringGt2NotNumeric: null.StringFrom("test"),
			},
		}
		expectedErr := "field 'StringGt2NotNumeric' is not a numeric"
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, true, actualErr != nil)
		assert.Equal(t, expectedErr, actualErr.Error())
	})
	t.Run("Should return error field is greater than", func(t *testing.T) {
		type mockInnerStruct struct {
			StringGt2Failed null.String `db:"StringGt2Failed" json:"StringGt2Failed" schema:"StringGt2Failed" validate:"gt=2"`
		}
		mockParam := struct {
			InnerStruct     mockInnerStruct
			SkipStringField string `db:"skip_string_field" json:"skip_string_field" schema:"skip_string_field" validate:"required"`
		}{
			InnerStruct: mockInnerStruct{
				StringGt2Failed: null.StringFrom("1"),
			},
		}
		expectedErr := "field 'StringGt2Failed' must be greater than 2"
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, true, actualErr != nil)
		assert.Equal(t, expectedErr, actualErr.Error())
	})
	t.Run("Should return error field is required", func(t *testing.T) {
		type mockInnerStruct struct {
			StringRequiredButNotAssign null.String `db:"StringRequiredButNotAssign" json:"StringRequiredButNotAssign,omitempty" validate:"required" update:"continue"`
		}
		mockParam := struct {
			InnerStruct mockInnerStruct
		}{
			InnerStruct: mockInnerStruct{},
		}
		expectedErr := "field value of StringRequiredButNotAssign is required field"
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, true, actualErr != nil)
		assert.Equal(t, expectedErr, actualErr.Error())
	})
	t.Run("Should return success field is skip because not assign", func(t *testing.T) {
		mockParam := struct {
			FieldSkipIfNull null.String `db:"FieldSkipIfNull" json:"FieldSkipIfNull,omitempty" validate:"omitempty" update:"skipIfNull"`
		}{}
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Should return success field is skip because fieldname is 'Valid'", func(t *testing.T) {
		mockParam := struct {
			Valid bool
		}{}
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Should return success field is skip because doesn't have 'validate' tag field", func(t *testing.T) {
		mockParam := struct {
			Name null.String
		}{}
		actualErr := nullValidator(http.MethodPatch, &mockParam)
		assert.Equal(t, false, actualErr != nil)
	})
}
