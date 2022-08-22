package binding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/go-playground/validator"
	"github.com/gorilla/schema"
)

const (
	ContentURLEncoded string = "application/x-www-form-urlencoded"
	ContentJSON       string = "application/json"
	ContentFormData   string = "multipart/form-data"
)

var (
	ErrInvalidContentType     = errors.New("Unrecognized content type")
	doHandleDecodeSchema      = decodeSchemaRequest
	readAllContent            = ioutil.ReadAll
	decodeJSON                = (*json.Decoder).Decode
	decodeSchema              = (*schema.Decoder).Decode
	doHandleNullValidator     = nullValidator
	validatorJSONStruct       = (*validator.Validate).Struct
	parseFormRequest          = (*http.Request).ParseForm
	parseMultiPartFormRequest = (*http.Request).ParseMultipartForm
)

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

// Bind parses the request body and bind the values to the passed interface,
// will also validate the value if it implements the Validation interface.
// Uses standard lib for json, uses github.com/gorilla/schema for url-encoded body
func Bind(r *http.Request, val interface{}) error {
	//ctx := r.Context()
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "Lib-Binding")
	//defer trc.Finish()
	if r.Method == http.MethodGet {
		if err := doHandleDecodeSchema(r, val); err != nil {
			return err
		}
		return nil
	}
	contentType := filterFlags(r.Header.Get("Content-Type"))

	if r.Method != http.MethodDelete {
		switch contentType {
		case ContentURLEncoded:
			err := parseFormRequest(r)
			if err != nil {
				return err
			}

			if err := doHandleDecodeSchema(r, val); err != nil {
				return err
			}

		case ContentJSON:
			body, err := readAllContent(r.Body)
			if err != nil {
				return err
			}

			ioReader := ioutil.NopCloser(bytes.NewBuffer(body))
			err = decodeJSON(json.NewDecoder(ioReader), val)
			if err != nil {
				return err
			}

			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		case ContentFormData:
			err := parseMultiPartFormRequest(r, 32<<20)
			if err != nil {
				return err
			}
			if err := doHandleDecodeSchema(r, val); err != nil {
				return err
			}
		case "text/plain":
			return nil
		default:
			return ErrInvalidContentType
		}
	}

	var reflectValue = reflect.ValueOf(val)

	if reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}

	if reflectValue.Kind() == reflect.Struct {
		if err := validatorJSONStruct(validatorJSON, val); err != nil {
			return err
		}
	}

	return nil
}

func decodeSchemaRequest(r *http.Request, val interface{}) error {
	decoder := schema.NewDecoder()
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
