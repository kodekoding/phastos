package api

import (
	"bytes"
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/gorilla/schema"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

var (
	validate                  = validator.New()
	decodeSchema              = (*schema.Decoder).Decode
	parseFormRequest          = (*http.Request).ParseForm
	parseMultiPartFormRequest = (*http.Request).ParseMultipartForm
	doHandleDecodeSchema      = decodeSchemaRequest
	readAllContent            = ioutil.ReadAll
	decodeJSON                = (*json.Decoder).Decode
)

const (
	ContentURLEncoded string = "application/x-www-form-urlencoded"
	ContentJSON       string = "application/json"
	ContentFormData   string = "multipart/form-data"

	ErrParsedBodyCode string = "ERROR_PARSING_BODY"
	ErrDecodeBodyCode string = "ERROR_DECODE_BODY"
)

func WriteJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(data)
	_, _ = w.Write(b)
}

type ValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

func ValidateStruct(data interface{}) []*ValidationError {
	var errors []*ValidationError
	err := validate.Struct(data)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ValidationError
			element.Field = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

func ArrContains[T string | int | float32](arr []T, val T) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}

func GetStructKey(field reflect.StructField) string {
	var key string
	key = field.Tag.Get("db")
	if key == "" {
		key = field.Tag.Get("json")
	}
	if key == "" {
		key = strings.ToLower(field.Name)
	}
	return key
}

func GetStructValue(value reflect.Value) interface{} {
	intc := value.Interface()
	switch intc.(type) {
	case int, int8, int16, int32, int64:
		return value.Int()
	case float32, float64:
		return value.Float()
	case bool:
		return value.Bool()
	case time.Time:
		return value.Interface().(time.Time).String()
	case uuid.UUID:
		return value.Interface().(uuid.UUID).String()
	default:
		return value.String()
	}
}

func GenerateQueryComponenFromStruct(model interface{}, skips []string) (string, []interface{}, string) {
	var fields []string
	var values []interface{}
	var binds []string
	v := reflect.ValueOf(model)
	for i := 0; i < v.NumField(); i++ {
		key := GetStructKey(v.Type().Field(i))
		value := GetStructValue(v.Field(i))
		if ArrContains(skips, key) {
			break
		}
		fields = append(fields, key)
		values = append(values, value)
		binds = append(binds, "$"+strconv.Itoa(len(values)))
	}
	return strings.Join(fields, ", "), values, strings.Join(binds, ", ")
}

// filterFlags is a utility to cleanly get a Content-Type header
// refer to https://github.com/gin-gonic/gin/blob/master/utils.go#L83
func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func decodeSchemaRequest(r *http.Request, val interface{}) error {
	decoder.IgnoreUnknownKeys(true)
	sourceDecode := r.Form

	if r.Method == http.MethodGet {
		sourceDecode = r.URL.Query()
	}

	if err := decodeSchema(decoder, val, sourceDecode); err != nil {
		return err
	}
	return nil
}

func getBodyFromJSON(r *http.Request, dest interface{}) error {
	body, err := readAllContent(r.Body)
	if err != nil {
		return err
	}

	ioReader := ioutil.NopCloser(bytes.NewBuffer(body))

	if err = decodeJSON(json.NewDecoder(ioReader), dest); err != nil {
		return BadRequest(err.Error(), ErrParsedBodyCode)
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return nil
}
