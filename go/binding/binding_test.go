package binding

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/common"

	"github.com/go-playground/validator"
	"github.com/gorilla/schema"
	"github.com/stretchr/testify/assert"
)

type testStructValidation struct {
	Name      string `json:"name" schema:"name" validate:"required"`
	DontParse string `json:"-" schema:"-"`
}

func TestBind(t *testing.T) {
	mockRequest := httptest.NewRequest(http.MethodGet, "http://tokopedia.com", nil)
	mockStructValue := struct {
		Name string
	}{
		Name: "test",
	}
	t.Run("Success Case GET", func(t *testing.T) {
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return nil
		}
		actualErr := Bind(mockRequest, map[string]interface{}{"test": "test"})
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Failed GET When decode schema request", func(t *testing.T) {
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, map[string]interface{}{"test": "test"})
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("success content-type text/plain", func(t *testing.T) {
		mockRequest.Method = http.MethodDelete
		mockRequest.Header.Set("Content-Type", "text/plain")

		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Success Case POST", func(t *testing.T) {
		mockRequest.Method = http.MethodPost
		mockRequest.Header.Set("Content-Type", ContentJSON)
		readAllContent = func(r io.Reader) ([]byte, error) {
			return nil, nil
		}
		decodeJSON = func(j *json.Decoder, i interface{}) error {
			return nil
		}
		validatorJSONStruct = func(v *validator.Validate, i interface{}) error {
			return nil
		}
		doHandleNullValidator = func(httpMethod string, structName interface{}, validationLists ...string) error {
			return nil
		}
		actualErr := Bind(mockRequest, mockStructValue)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("JSON - Should return error when handle validatprJSONStruct", func(t *testing.T) {
		validatorJSONStruct = func(v *validator.Validate, i interface{}) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("JSON - Should return error when decode JSON", func(t *testing.T) {

		decodeJSON = func(j *json.Decoder, i interface{}) error {
			return common.ErrPatch
		}

		actualErr := Bind(mockRequest, mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("JSON - Should return error when read all content", func(t *testing.T) {

		readAllContent = func(r io.Reader) ([]byte, error) {
			return nil, common.ErrPatch
		}

		actualErr := Bind(mockRequest, mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("URL encode - Success", func(t *testing.T) {
		mockRequest.Header.Set("Content-Type", ContentURLEncoded)
		parseFormRequest = func(h *http.Request) error {
			return nil
		}
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return nil
		}
		validatorJSONStruct = func(v *validator.Validate, i interface{}) error {
			return nil
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("URL encode - Should return error when decode schema", func(t *testing.T) {
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("URL encode - Should return error when decode schema", func(t *testing.T) {
		parseFormRequest = func(h *http.Request) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("Form Data - Success", func(t *testing.T) {
		mockRequest.Method = http.MethodPost
		mockRequest.Header.Set("Content-Type", ContentFormData)
		parseMultiPartFormRequest = func(h *http.Request, i int64) error {
			return nil
		}
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return nil
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Form Data - Should return error when decode schema", func(t *testing.T) {
		doHandleDecodeSchema = func(r *http.Request, val interface{}) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("Form Data - Should return error when decode schema", func(t *testing.T) {
		parseMultiPartFormRequest = func(h *http.Request, i int64) error {
			return common.ErrPatch
		}
		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
	t.Run("undefined content-type", func(t *testing.T) {
		mockRequest.Header.Set("Content-Type", "test")

		actualErr := Bind(mockRequest, &mockStructValue)
		assert.Equal(t, true, actualErr != nil)
	})
}

//func TestBind(t *testing.T) {
//	type args struct {
//		r   *http.Request
//		val interface{}
//	}
//	tests := []struct {
//		name    string
//		args    args
//		patch   func()
//		wantErr error
//		wantVal *testStructValidation
//	}{
//		{
//			name: "should successfully bind the json payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentJSON},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader(`{"name": "wildanjing"}`)),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: nil,
//			wantVal: &testStructValidation{Name: "wildanjing"},
//		},
//		{
//			name: "should successfully bind the url-encoded payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentURLEncoded},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("name=wildanjing")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: nil,
//			wantVal: &testStructValidation{Name: "wildanjing"},
//		},
//		{
//			name: "should successfully bind the url-encoded payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentURLEncoded},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("name=wildanjing")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: nil,
//			wantVal: &testStructValidation{Name: "wildanjing"},
//		},
//		{
//			name: "should return error on ParseForm because of malformed payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentURLEncoded},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("%%%")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: url.EscapeError("%%%"),
//			wantVal: &testStructValidation{},
//		},
//		{
//			name: "should return error on decoder.Decode",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentURLEncoded},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			patch: func() {
//				decoder := schema.NewDecoder()
//				var guard *mpatch.Patch
//				guard, _ = mpatch.PatchInstanceMethodByName(reflect.TypeOf(decoder), "Decode", func(dec *schema.Decoder, dst interface{}, src map[string][]string) error {
//					defer guard.Unpatch()
//					return errors.New("error occurred")
//				})
//			},
//			wantErr: errors.New("error occurred"),
//			wantVal: &testStructValidation{},
//		},
//		{
//			name: "should return error on json.Decoder",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentJSON},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("{}")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			patch: func() {
//				decoder := &json.Decoder{}
//				var guard *mpatch.Patch
//				guard, _ = mpatch.PatchInstanceMethodByName(reflect.TypeOf(decoder), "Decode", func(dec *json.Decoder, dst interface{}) error {
//					defer guard.Unpatch()
//					return errors.New("error occurred")
//				})
//			},
//			wantErr: errors.New("error occurred"),
//			wantVal: &testStructValidation{},
//		},
//		{
//			name: "should return validation error on empty url-encode payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentURLEncoded},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: errors.New("test"),
//			wantVal: &testStructValidation{},
//		},
//		{
//			name: "should return validation error on empty json payload",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{ContentJSON + ";utf-8"},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("{}")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: errors.New("test"),
//			wantVal: &testStructValidation{},
//		},
//		{
//			name: "should return unrecognized content type",
//			args: args{
//				r: &http.Request{
//					Header: http.Header{
//						"Content-Type": []string{"wildanjing"},
//					},
//					Body:   ioutil.NopCloser(strings.NewReader("{}")),
//					Method: "POST",
//				},
//				val: &testStructValidation{},
//			},
//			wantErr: ErrInvalidContentType,
//			wantVal: &testStructValidation{},
//		},
//	}
//	for _, tt := range tests {
//		if tt.patch != nil {
//			tt.patch()
//		}
//		t.Run(tt.name, func(t *testing.T) {
//			if err := Bind(tt.args.r, tt.args.val); err != nil {
//				assert.Equal(t, tt.wantErr, err)
//			}
//			assert.Equal(t, tt.wantVal, tt.args.val)
//		})
//	}
//}

func Test_decodeSchemaRequest(t *testing.T) {
	mockRequest := httptest.NewRequest(http.MethodGet, "http://tokopedia.com", nil)
	t.Run("Success Case", func(t *testing.T) {
		decodeSchema = func(s *schema.Decoder, i interface{}, m map[string][]string) error {
			return nil
		}
		actualErr := decodeSchemaRequest(mockRequest, 1)
		assert.Equal(t, false, actualErr != nil)
	})
	t.Run("Should return error when decode schema", func(t *testing.T) {
		decodeSchema = func(s *schema.Decoder, i interface{}, m map[string][]string) error {
			return common.ErrPatch
		}
		actualErr := decodeSchemaRequest(mockRequest, 1)
		assert.Equal(t, true, actualErr != nil)
	})
}

func Test_filterFlags(t *testing.T) {
	t.Run("Should return content", func(t *testing.T) {
		expected := "hallo"
		actual := filterFlags(expected)
		assert.Equal(t, expected, actual)
	})

	t.Run("Should return until specific char", func(t *testing.T) {
		param := "hallo 123"
		expected := "hallo"
		actual := filterFlags(param)
		assert.Equal(t, expected, actual)
	})
}
