package helper

import (
	"bytes"
	"embed"
	"github.com/pkg/errors"
	"html/template"
	"io"
)

func ParseTemplate(embedFS embed.FS, file string, args interface{}) ([]byte, error) {
	var templateValue []byte
	var err error
	if args != nil {

		// read the block-kit definition as a go template
		var tpl bytes.Buffer
		t, err := template.ParseFS(embedFS, file)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ParseFS")
		}

		// we render the view
		err = t.Execute(&tpl, args)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ExecuteTemplate")
		}
		templateValue, err = io.ReadAll(&tpl)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ReadAllTemplate")
		}
	} else {
		if templateValue, err = embedFS.ReadFile(file); err != nil {
			return nil, errors.Wrap(err, "phastos.go.helper.template.ParseTemplate.ReadFile")
		}
	}

	return templateValue, nil
}
