package binding

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
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

func nullValidator(httpMethod string, structName interface{}, validationLists ...string) error {
	reflectVal := reflect.ValueOf(structName)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	refType := reflectVal.Type()
	valLength := len(validationLists)
	for i := 0; i < reflectVal.NumField(); i++ {
		fieldType := refType.Field(i)
		dataType := fieldType.Type.String()
		fieldName := fieldType.Name
		fieldExisting := reflectVal.Field(i)
		isNullStruct := strings.Contains(dataType, "null.")

		if fieldExisting.Kind() == reflect.Struct && !isNullStruct {
			// for embedded struct non-"null."
			err := nullValidator(httpMethod, fieldExisting.Interface())
			if err != nil {
				return err
			}
		}
		if (validationLists == nil && !isNullStruct) || fieldName == "Valid" {
			continue
		} else if valLength > 0 {
			errChan := make(chan error, 0)
			colName := validationLists[valLength-1]

			validationLists = validationLists[0 : valLength-1]
			go func(field reflect.Value, chanErr chan<- error) {
				validValue := reflectVal.Field(1).Interface().(bool)
				updateValidation := validationLists[0]
				value := field.Interface()
				for _, validate := range validationLists {
					if httpMethod == http.MethodPatch && updateValidation == "skipIfNull" {
						break
					}
					if validate == "required" || validate == "omitempty" {
						if !validValue {
							errs := fmt.Errorf("field value of %s is required field", colName)
							chanErr <- errs
							break
						}

					}
					if strings.Contains(validate, "gt") {
						splitValid := strings.Split(validate, "=")
						gt, _ := strconv.Atoi(splitValid[1])
						intVal, ok := value.(int)
						if !ok {
							var err error
							intVal, err = strconv.Atoi(value.(string))
							if err != nil {
								errs := fmt.Errorf("field '%s' is not a numeric", colName)
								chanErr <- errs
								break
							}
						}
						if intVal <= gt {
							errs := fmt.Errorf("field '%s' must be greater than %d", colName, gt)
							chanErr <- errs
							break
						}
					}
				}
				chanErr <- nil
				close(chanErr)
			}(fieldExisting, errChan)

			for err := range errChan {
				if err != nil {
					return err
				}
			}
			return nil
		}
		validateOptions, hasValidateTag := fieldType.Tag.Lookup("validate")
		if !hasValidateTag {
			continue
		}

		var validationList []string
		updateTagValue, hasUpdateTag := fieldType.Tag.Lookup("update")
		if httpMethod == http.MethodPatch && hasUpdateTag {
			validationList = append(validationList, updateTagValue)
		}

		validationList = append(validationList, strings.Split(validateOptions, ",")...)
		schemeName, hasSchemaTag := fieldType.Tag.Lookup("schema")
		fieldName = schemeName

		if !hasSchemaTag {
			jsonTag := fieldType.Tag.Get("json")
			jsonName := jsonTag[:strings.IndexRune(jsonTag, ',')]
			fieldName = jsonName
		}
		validationList = append(validationList, fieldName)
		err := nullValidator(httpMethod, fieldExisting.Interface(), validationList...)
		if err != nil {
			return err
		}
	}
	return nil
}
