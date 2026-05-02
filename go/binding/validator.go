package binding

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator"
)

var (
	validatorJSON *validator.Validate
	validatorURL  *validator.Validate
)

func init() {
	validatorJSON = validator.New()
	validatorJSON.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	validatorURL = validator.New()
	validatorURL.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("schema"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
