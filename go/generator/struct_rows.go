package generator

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

func structFieldToString(value interface{}) string {
	if value == nil {
		return ""
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
		value = rv.Interface()
	}
	if valuer, ok := value.(driver.Valuer); ok {
		resolved, err := valuer.Value()
		if err != nil || resolved == nil {
			return ""
		}
		return fmt.Sprintf("%v", resolved)
	}
	return fmt.Sprintf("%v", value)
}

func structToColumnMap(v reflect.Value) map[string]string {
	out := make(map[string]string)
	collectDBTaggedFields(v, out)
	return out
}

func collectDBTaggedFields(v reflect.Value, out map[string]string) {
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldValue := v.Field(i)

		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			if _, isValuer := fieldValue.Interface().(driver.Valuer); !isValuer {
				collectDBTaggedFields(fieldValue, out)
				continue
			}
		}

		dbTag := field.Tag.Get("db")
		if dbTag == "" || dbTag == "-" {
			continue
		}
		dbTag = strings.Split(dbTag, ",")[0]
		out[dbTag] = structFieldToString(fieldValue.Interface())
	}
}

func StructRows(columns []string, slice interface{}) ([][]string, error) {
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return nil, errors.New("StructRows: input must be a slice")
	}
	if len(columns) == 0 {
		return nil, errors.New("StructRows: columns must not be empty")
	}

	n := v.Len()
	rows := make([][]string, n)
	for i := 0; i < n; i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				rows[i] = make([]string, len(columns))
				continue
			}
			elem = elem.Elem()
		}
		colMap := structToColumnMap(elem)
		row := make([]string, len(columns))
		for ci, col := range columns {
			row[ci] = colMap[col]
		}
		rows[i] = row
	}
	return rows, nil
}
