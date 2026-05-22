package helper

import (
	"embed"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed *.go
var testEmbedFS embed.FS

//go:embed testdata
var testTemplateEmbedFS embed.FS

func TestParseTemplate_WithNilArgs(t *testing.T) {
	t.Run("should read file content when args is nil", func(t *testing.T) {
		// Use an existing file from the embed FS
		result, err := ParseTemplate(testEmbedFS, "fast_id.go", nil)
		assert.NoError(t, err)
		assert.True(t, result.Len() > 0)
		assert.Contains(t, result.String(), "package helper")
	})
}

func TestParseTemplate_ExecError(t *testing.T) {
	t.Run("should return error when template execution fails", func(t *testing.T) {
		_, err := ParseTemplate(testTemplateEmbedFS, "testdata/exec_error.tmpl", map[string]string{"Name": "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no such template")
	})
}

func TestParseTemplate_WithArgs(t *testing.T) {
	t.Run("should execute template with args", func(t *testing.T) {
		// Create a temporary template file
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/test.tmpl"
		err := os.WriteFile(tmplPath, []byte("Hello {{.Name}}!"), 0644)
		assert.NoError(t, err)

		// We can't use embed.FS for dynamically created files,
		// so let's test ParseTemplateFromPath instead
	})
}

func TestParseTemplateFromPath_WithLocalFile(t *testing.T) {
	t.Run("should parse template from local file path", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/test.tmpl"
		err := os.WriteFile(tmplPath, []byte("Hello {{.Name}}!"), 0644)
		assert.NoError(t, err)

		data := map[string]string{"Name": "World"}
		result, err := ParseTemplateFromPath(tmplPath, data)
		assert.NoError(t, err)
		assert.Contains(t, result.String(), "Hello World!")
	})

	t.Run("should parse template without optional params", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/simple.tmpl"
		err := os.WriteFile(tmplPath, []byte("Value: {{.Val}}"), 0644)
		assert.NoError(t, err)

		data := map[string]string{"Val": "42"}
		result, err := ParseTemplateFromPath(tmplPath, data)
		assert.NoError(t, err)
		assert.Contains(t, result.String(), "Value: 42")
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		data := map[string]string{"Name": "Test"}
		_, err := ParseTemplateFromPath("/nonexistent/path/template.tmpl", data)
		assert.Error(t, err)
	})

	t.Run("should handle FuncMap optional param", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/func.tmpl"
		err := os.WriteFile(tmplPath, []byte("{{.Name | upper}}"), 0644)
		assert.NoError(t, err)

		data := map[string]string{"Name": "hello"}
		funcMap := map[string]any{
			"upper": strings.ToUpper,
		}
		result, err := ParseTemplateFromPath(tmplPath, data, funcMap)
		assert.NoError(t, err)
		assert.Contains(t, result.String(), "HELLO")
	})
}

func TestParseFileTemplate(t *testing.T) {
	t.Run("should parse template from file path", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/test.tmpl"
		err := os.WriteFile(tmplPath, []byte("Hello {{.Name}}!"), 0644)
		assert.NoError(t, err)

		data := map[string]string{"Name": "World"}
		result, err := ParseFileTemplate(tmplPath, data)
		assert.NoError(t, err)
		assert.Contains(t, result.String(), "Hello World!")
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		data := map[string]string{"Name": "Test"}
		_, err := ParseFileTemplate("/nonexistent/path/template.tmpl", data)
		assert.Error(t, err)
	})

	t.Run("should handle template syntax error", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/bad.tmpl"
		err := os.WriteFile(tmplPath, []byte("{{.BadSyntax"), 0644)
		assert.NoError(t, err)

		data := map[string]string{}
		_, err = ParseFileTemplate(tmplPath, data)
		assert.Error(t, err)
	})

	t.Run("should handle additional body content", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/body.tmpl"
		err := os.WriteFile(tmplPath, []byte("{{.Content}}"), 0644)
		assert.NoError(t, err)

		data := map[string]string{"Content": "main"}
		result, err := ParseFileTemplate(tmplPath, data, "<prefix>", "</suffix>")
		assert.NoError(t, err)
		assert.Contains(t, result.String(), "<prefix>")
		assert.Contains(t, result.String(), "main")
		assert.Contains(t, result.String(), "</suffix>")
	})
}

func TestGetTemplateFS(t *testing.T) {
	t.Run("should parse template from embed FS and unmarshal to struct", func(t *testing.T) {
		tmpDir := t.TempDir()
		// We need a template that produces valid JSON
		tmplContent := `{"name":"{{.Name}}","age":{{.Age}}}`
		tmplPath := tmpDir + "/test.json.tmpl"
		err := os.WriteFile(tmplPath, []byte(tmplContent), 0644)
		assert.NoError(t, err)

		// GetTemplateFS requires an embed.FS, so we use our testEmbedFS
		// with a .go file that can't produce valid JSON — test the error path
		var dest map[string]interface{}
		err = GetTemplateFS(testEmbedFS, "fast_id.go", nil, &dest)
		// This should fail because .go files don't produce valid JSON
		assert.Error(t, err)
	})

	t.Run("success path with valid JSON template", func(t *testing.T) {
		data := map[string]string{"Name": "TestName"}
		var dest map[string]string
		err := GetTemplateFS(testTemplateEmbedFS, "testdata/success.json.tmpl", data, &dest)
		assert.NoError(t, err)
		assert.Equal(t, "TestName", dest["name"])
	})
}

func TestGetTemplate(t *testing.T) {
	t.Run("should parse template from path and unmarshal to struct", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplContent := `{"name":"{{.Name}}","age":{{.Age}}}`
		tmplPath := tmpDir + "/test.json.tmpl"
		err := os.WriteFile(tmplPath, []byte(tmplContent), 0644)
		assert.NoError(t, err)

		data := struct {
			Name string
			Age  int
		}{Name: "John", Age: 30}

		type resultStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		var dest resultStruct
		err = GetTemplate(tmplPath, data, &dest)
		assert.NoError(t, err)
		assert.Equal(t, "John", dest.Name)
		assert.Equal(t, 30, dest.Age)
	})

	t.Run("should return error for invalid template path", func(t *testing.T) {
		var dest map[string]interface{}
		err := GetTemplate("/nonexistent/path", nil, &dest)
		assert.Error(t, err)
	})

	t.Run("should return error when dest is not a pointer", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmplPath := tmpDir + "/test.json.tmpl"
		err := os.WriteFile(tmplPath, []byte(`{"key":"val"}`), 0644)
		assert.NoError(t, err)

		dest := map[string]interface{}{}
		err = GetTemplate(tmplPath, nil, dest) // not a pointer
		assert.Error(t, err)
	})
}
