package helper

import (
	"regexp"

	"github.com/iancoleman/strcase"
)

// ToCamelCase - will be convert string to camel case format
//	example:"AnyKind of_string"
//	result:"AnyKindOfString"
func ToCamelCase(str string) string {
	return strcase.ToCamel(str)
}

// ToLowerCamelCase - will be convert string to camel case format with lower case at first letter
//	example:"AnyKind of_string"
//	result:"anyKindOfString"
func ToLowerCamelCase(str string) string {
	return strcase.ToLowerCamel(str)
}

// ToSnakeCase - will be convert string to snake case format
//	example:"AnyKind of_string"
//	result:"any_kind_of_string"
func ToSnakeCase(s string) string {
	return strcase.ToSnake(s)
}

func TrimDuplicatedSpace(str string) string {
	space := regexp.MustCompile(`\s+`)
	return space.ReplaceAllString(str, " ")
}
