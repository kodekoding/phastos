package slack

import (
	"bytes"
	"embed"
	"encoding/json"
	"github.com/pkg/errors"
	"html/template"
	"io"
)

func GetTemplate(embedFS embed.FS, file string, args, destStruct interface{}) error {

	var templateValue []byte
	var err error
	if args != nil {

		// read the block-kit definition as a go template
		var tpl bytes.Buffer
		t, err := template.ParseFS(embedFS, file)
		if err != nil {
			return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ParseFS")
		}

		// we render the view
		err = t.Execute(&tpl, args)
		if err != nil {
			return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ExecuteTemplate")
		}
		templateValue, err = io.ReadAll(&tpl)
		if err != nil {
			return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ReadAllTemplate")
		}
	} else {
		if templateValue, err = embedFS.ReadFile(file); err != nil {
			return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.ReadFile")
		}

	}

	if err = json.Unmarshal(templateValue, destStruct); err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.UnmarshalToStruct")
	}

	return nil
}
