package slack

import (
	"bytes"
	"embed"
	"encoding/json"
	"github.com/pkg/errors"
	"html/template"
	"io"
)

func GetTemplate(file string, args, destStruct interface{}) error {

	var tpl bytes.Buffer
	var embedFS embed.FS

	// read the block-kit definition as a go template
	t, err := template.ParseFS(embedFS, file)
	if err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ParseFS")
	}

	// we render the view
	err = t.Execute(&tpl, args)
	if err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ExecuteTemplate")
	}

	value, err := io.ReadAll(&tpl)
	if err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ReadAllTemplate")
	}
	if err = json.Unmarshal(value, destStruct); err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.UnmarshalToStruct")
	}

	return nil
}
