package helper

import (
	"bytes"
	"embed"
	"github.com/rs/zerolog/log"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func ParseTemplate(embedFS embed.FS, file string, args interface{}, additionalBodyContent ...string) (bytes.Buffer, error) {
	var templateValue []byte
	var err error
	var tpl bytes.Buffer
	if args != nil {

		// read the block-kit definition as a go template
		t, err := template.ParseFS(embedFS, file)
		if err != nil {
			return tpl, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ParseFS")
		}

		for _, content := range additionalBodyContent {
			tpl.Write([]byte(content))
		}

		// we render the view
		err = t.Execute(&tpl, args)
		if err != nil {
			return tpl, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ExecuteTemplate")
		}

	} else {
		if templateValue, err = embedFS.ReadFile(file); err != nil {
			return tpl, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ReadFile")
		}

		tpl.Write(templateValue)
	}

	return tpl, nil
}

func ParseFileTemplate(filePath string, args interface{}, additionalBodyContent ...string) (bytes.Buffer, error) {
	var err error
	var tpl bytes.Buffer
	t, err := template.ParseFiles(filePath)
	if err != nil {
		return tpl, errors.Wrap(err, "phastos.go.helper.template.ParseFileTemplate.ParseFiles")
	}
	for _, content := range additionalBodyContent {
		tpl.Write([]byte(content))
	}
	if err = t.Execute(&tpl, args); err != nil {
		return tpl, errors.Wrap(err, "phastos.go.helper.template.ParseFileTemplate.ExecuteTemplate")
	}

	return tpl, nil
}

func ParseTemplateFromPath(filePath string, data any, optionalParams ...any) (bytes.Buffer, error) {
	var err error
	var result bytes.Buffer
	name := filepath.Base(filePath)
	tmpl := template.New(name)

	// checking optional params
	if optionalParams != nil && len(optionalParams) > 0 {
		for _, param := range optionalParams {
			switch value := param.(type) {
			case string:
				result.WriteString(value)
			case template.FuncMap:
				tmpl.Funcs(value)
			default:
				log.Warn().Any("val", value).Msg("Undefined optional params data type")
			}
		}
	}

	isURLPath := strings.Contains(filePath, "http")

	var templateContent strings.Builder
	// getting template contents
	if isURLPath {
		// if templatePath is url, ex: https://................/file.html
		resp, err := http.Get(filePath)
		if err != nil {
			err = errors.Wrap(err, "phastos.helper.template.ParseTemplateFromPath.GetTemplateFromURL")
			return result, err
		}
		defer resp.Body.Close()

		htmlContent, err := io.ReadAll(resp.Body)
		if err != nil {
			err = errors.Wrap(err, "phastos.helper.template.ParseTemplateFromPath.ReadBodyResponseHTML")
			return result, err
		}
		templateContent.Write(htmlContent)
	} else {
		// if templatePath is local path, ex: /tmp/templates/file.html
		contentByte, err := os.ReadFile(filePath)
		if err != nil {
			err = errors.Wrap(err, "phastos.helper.template.ParseTemplateFromPath.ReadFileFromLocalPath")
			return result, err
		}
		templateContent.Write(contentByte)

	}
	parsedTemplate, err := tmpl.Parse(templateContent.String())
	if err != nil {
		return result, errors.Wrap(err, "phastos.helper.template.ParseTemplateFromPath.ParseContent")
	}

	if err = parsedTemplate.Execute(&result, data); err != nil {
		return result, errors.Wrap(err, "phastos.helper.template.ParseTemplateFromPath.Execute")
	}
	return result, nil
}
