package helper

import (
	"embed"
	"encoding/json"
	"github.com/pkg/errors"
)

func GetTemplate(embedFS embed.FS, file string, args, destStruct interface{}) error {

	templateValue, err := ParseTemplate(embedFS, file, args)
	if err != nil {
		return errors.Wrap(err, "phastos.go.helper.slack.GetTemplate.ParseTemplate")
	}

	if err = json.Unmarshal(templateValue, destStruct); err != nil {
		return errors.Wrap(err, "phastos.go.third_party.slack.templating.GetTemplate.UnmarshalToStruct")
	}

	return nil
}
