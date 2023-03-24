package app

import (
	"encoding/json"
	"github.com/go-playground/validator"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

var validate = validator.New()

func WriteJson(w http.ResponseWriter, data interface{}) {
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

//func GetUserContext(ctx context.Context) *UserContext {
//	return ctx.Value(USER_CONTEXT_KEY).(*UserContext)
//}
//
//func StringToDBConnection(conn string) DBConnection {
//	temp1 := strings.Split(conn, "://")
//	dialect := temp1[0]
//
//	temp2 := strings.Split(temp1[1], "@")
//
//	temp3 := strings.Split(temp2[0], ":")
//	username := temp3[0]
//	password := temp3[1]
//
//	temp4 := strings.Split(temp2[1], "/")
//	database := temp4[1]
//
//	temp5 := strings.Split(temp4[0], ":")
//	host := temp5[0]
//	port := temp5[1]
//
//	return DBConnection{
//		Dialect:  dialect,
//		Username: username,
//		Password: password,
//		Host:     host,
//		Port:     port,
//		Database: database,
//	}
//}
