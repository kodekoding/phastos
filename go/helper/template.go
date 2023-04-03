package helper

import (
	"bytes"
	"embed"
	"github.com/pkg/errors"
	"html/template"
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
