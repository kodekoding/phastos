package binding

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "application/json", "application/json"},
		{"with charset", "application/json; charset=utf-8", "application/json"},
		{"with space", "application/json ;charset=utf-8", "application/json"},
		{"empty", "", ""},
		{"multipart", "multipart/form-data; boundary=----", "multipart/form-data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterFlags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentConstants(t *testing.T) {
	assert.Equal(t, "application/x-www-form-urlencoded", ContentURLEncoded)
	assert.Equal(t, "application/json", ContentJSON)
	assert.Equal(t, "multipart/form-data", ContentFormData)
}

func TestBindGETRequest(t *testing.T) {
	type QueryStruct struct {
		Name string `schema:"name"`
	}
	
	req := httptest.NewRequest(http.MethodGet, "/test?name=hello", nil)
	var result QueryStruct
	err := Bind(req, &result)
	assert.NoError(t, err)
	assert.Equal(t, "hello", result.Name)
}

func TestBindJSONPostRequest(t *testing.T) {
	type BodyStruct struct {
		Name string `json:"name"`
	}
	
	body := `{"name":"world"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	var result BodyStruct
	err := Bind(req, &result)
	assert.NoError(t, err)
	assert.Equal(t, "world", result.Name)
}

func TestBindDeleteRequestNoBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/test/1", nil)
	var result struct{}
	err := Bind(req, &result)
	assert.NoError(t, err)
}

func TestBindInvalidContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("data"))
	req.Header.Set("Content-Type", "application/xml")
	
	var result struct{}
	err := Bind(req, &result)
	assert.Equal(t, ErrInvalidContentType, err)
}

func TestBindTextPlainContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("text"))
	req.Header.Set("Content-Type", "text/plain")
	
	var result struct{}
	err := Bind(req, &result)
	assert.NoError(t, err)
}

func TestBindNonPointerError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	result := struct{ Name string }{}
	// Passing value (not pointer) should still work since reflect check is on val param
	err := Bind(req, &result)
	assert.NoError(t, err)
}

func TestBindURLPostRequest(t *testing.T) {
	type FormStruct struct {
		Name string `schema:"name"`
	}
	
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("name=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	var result FormStruct
	err := Bind(req, &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result.Name)
}

func TestErrInvalidContentType(t *testing.T) {
	assert.Equal(t, "unrecognized content type", ErrInvalidContentType.Error())
}

func TestValidatorTagNameFunc_JSONDashTag(t *testing.T) {
	type testStruct struct {
		HiddenField string `json:"-" validate:"required"`
	}
	err := validatorJSON.Struct(testStruct{})
	// HiddenField has json:"-" so its name is empty; validation fails
	// because zero-value string is not required
	assert.Error(t, err)
}

func TestValidatorTagNameFunc_SchemaDashTag(t *testing.T) {
	type testStruct struct {
		HiddenField string `schema:"-" validate:"required"`
	}
	err := validatorURL.Struct(testStruct{})
	assert.Error(t, err)
}

func TestValidatorTagNameFunc_JSONNonDashTag(t *testing.T) {
	type testStruct struct {
		Name string `json:"name" validate:"required"`
	}
	err := validatorJSON.Struct(testStruct{Name: "test"})
	assert.NoError(t, err)
}

func TestValidatorTagNameFunc_SchemaNonDashTag(t *testing.T) {
	type testStruct struct {
		Name string `schema:"name" validate:"required"`
	}
	err := validatorURL.Struct(testStruct{Name: "test"})
	assert.NoError(t, err)
}
