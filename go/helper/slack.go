package helper

import (
	"embed"
	"encoding/json"
	"github.com/pkg/errors"
)

func GetTemplateFS(embedFS embed.FS, file string, args, destStruct interface{}) error {

	templateValue, err := ParseTemplate(embedFS, file, args)
	if err != nil {
		return errors.Wrap(err, "phastos.go.helper.slack.GetTemplate.ParseTemplate")
	}

	if err = json.Unmarshal([]byte(templateValue.String()), destStruct); err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.UnmarshalToStruct")
	}

	return nil
}

func GetTemplate(file string, args, destStruct interface{}) error {

	templateValue, err := ParseTemplateFromPath(file, args)
	if err != nil {
		return errors.Wrap(err, "phastos.go.helper.slack.GetTemplate.ParseTemplate")
	}

	if err = json.Unmarshal([]byte(templateValue.String()), destStruct); err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.UnmarshalToStruct")
	}

	return nil
}
