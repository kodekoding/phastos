package helper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/volatiletech/null"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/log"
)

func ConstructColNameAndValueBulk(ctx context.Context, arrayOfData interface{}) (*database.CUDConstructData, error) {
	reflectVal := reflect.ValueOf(arrayOfData)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Slice {
		return nil, errors.New("second parameter should be Slice/Array")
	}

	totalData := reflectVal.Len()

	wg := new(sync.WaitGroup)
	mtx := new(sync.Mutex)

	maxLen := 0
	counterMaxLen := 0
	var columns []string
	var columnValues []interface{}
	var arrayOfValues [][]interface{}

	mapBulkValues := make(map[string][]interface{})
	for i := 0; i < totalData; i++ {
		wg.Add(1)
		data := reflect.Indirect(reflectVal.Index(i))
		go func(field reflect.Value, idx int) {
			defer func() {
				mtx.Unlock()
				wg.Done()
			}()
			cols, vals := readField(ctx, field)
			mtx.Lock()
			if counterMaxLen == 0 {
				columns = cols
			}
			for x, column := range cols {
				mapBulkValues[column] = append(mapBulkValues[column], vals[x])
			}
			totalCols := len(cols)
			if totalCols > maxLen {
				counterMaxLen++
				maxLen = totalCols
			}
			arrayOfValues = append(arrayOfValues, vals)
			columnValues = append(columnValues, vals...)
		}(data, i)
	}

	wg.Wait()
	if counterMaxLen > 1 {
		return nil, errors.New("length of each element is different")
	}

	errChan := make(chan error)
	go func(mapData map[string][]interface{}) {
		maxLen := 0
		counterMaxLen := 0
		for _, data := range mapData {
			jumlahData := len(data)
			if jumlahData != maxLen {
				counterMaxLen++
				maxLen = jumlahData
			}

			if counterMaxLen > 1 {
				errChan <- errors.New("length of each field is different")
			}
		}
		errChan <- nil
	}(mapBulkValues)

	if gotErr := <-errChan; gotErr != nil {
		return nil, fmt.Errorf("got error: %s", gotErr.Error())
	}

	var listOfBulkValues []string

	for _, values := range arrayOfValues {
		listOfBulkValues = append(listOfBulkValues, fmt.Sprintf("(?%s)", strings.Repeat(",?", len(values)-1)))
	}

	result := new(database.CUDConstructData)
	result.ColsInsert = strings.Join(columns, ",")
	result.BulkValuesInsert = strings.Join(listOfBulkValues, ",")
	result.Values = columnValues
	return result, nil
}

func ConstructColNameAndValue(ctx context.Context, structName interface{}, isNullStruct ...bool) ([]string, []interface{}) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "Helper-ConstructColNameAndValue")
	//defer trc.Finish()
	reflectVal := reflect.ValueOf(structName)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Struct {
		log.Errorln("second parameter should be struct")
		return nil, nil
	}

	cols, values := readField(ctx, reflectVal, isNullStruct...)
	return cols, values
}

func readField(_ context.Context, reflectVal reflect.Value, isNullStruct ...bool) ([]string, []interface{}) {
	refType := reflectVal.Type()
	var values []interface{}
	var cols []string
	//var partOfMainCols, hasOptionalParam bool
	//if isColMain != nil && len(isColMain) > 0 {
	//	partOfMainCols = isColMain[0]
	//	hasOptionalParam = true
	//}
	var containsNullStruct bool
	if isNullStruct != nil {
		containsNullStruct = isNullStruct[0]
	}
	numField := reflectVal.NumField()
	for i := 0; i < numField; i++ {
		field := reflectVal.Field(i)

		value := field.Interface()
		fieldType := refType.Field(i)
		colName := fieldType.Tag.Get("db")
		if colName == "-" {
			continue
		} else if colName == "id" {
			if number := value.(int); number == 0 {
				continue
			}
		}
		colTagVal, hasColTag := fieldType.Tag.Lookup("col")
		if hasColTag && colTagVal == "pk" {
			continue
		}
		fieldTypeData := fieldType.Type.String()
		nullStruct := strings.Contains(fieldTypeData, "null.")

		if nullStruct {
			containsNullStruct = true
		}

		if field.Kind() == reflect.Struct {

			embeddedCols, embeddedVals := ConstructColNameAndValue(nil, field.Interface(), containsNullStruct)

			if nullStruct && embeddedVals != nil {
				cols = append(cols, colName)
			} else {
				cols = append(cols, embeddedCols...)
			}
			values = append(values, embeddedVals...)

			continue
		}

		if fieldType.Name == "Valid" {
			if !(value.(bool)) {

				cols = nil
				values = nil
			}
			continue
		}

		switch field.Kind() {
		case reflect.String:
			if str := value.(string); str == "null" {
				value = null.String{}
			} else if str == "" {
				continue
			}
		}

		cols = append(cols, colName)
		values = append(values, value)
	}
	return cols, values
}

func ConstructColNameAndValueForUpdate(_ context.Context, structName interface{}, anotherValues ...interface{}) *database.CUDConstructData {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "Helper-ConstructColNameAndValueForUpdate")
	//defer trc.Finish()
	cols, values := ConstructColNameAndValue(nil, structName)
	var columns string
	columns = strings.Join(cols, ",")
	// change cols list with suffix '=?' using go routine
	mutex := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	colLength := len(cols)
	for i := 0; i < colLength; i++ {
		wg.Add(1)
		go func(col *string, wg *sync.WaitGroup, mtx *sync.Mutex) {
			mtx.Lock()
			*col = *col + "=?"
			mtx.Unlock()
			wg.Done()
		}(&cols[i], wg, mutex)
	}

	wg.Wait()

	if anotherValues != nil {
		values = append(values, anotherValues...)
	}
	return &database.CUDConstructData{
		Cols:       cols,
		ColsInsert: columns,
		Values:     values,
	}
}

func GenerateSelectCols(ctx context.Context, source interface{}, isNullStruct ...bool) []string {
	reflectVal := reflect.ValueOf(source)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	refType := reflectVal.Type()
	var cols []string

	var containsNullStruct bool
	if isNullStruct != nil {
		containsNullStruct = isNullStruct[0]
	}
	for i := 0; i < reflectVal.NumField(); i++ {
		field := reflectVal.Field(i)

		value := field.Interface()
		fieldType := refType.Field(i)
		fieldName := ToSnakeCase(fieldType.Name)
		val, exist := fieldType.Tag.Lookup("db")
		if exist {
			fieldName = val
		}
		fieldTypeData := fieldType.Type.String()
		if strings.Contains(fieldTypeData, "null.") {
			cols = append(cols, fieldName)
			continue
		}
		if field.Kind() == reflect.Struct {
			embeddedCols := GenerateSelectCols(ctx, value, containsNullStruct)
			cols = append(cols, embeddedCols...)
			continue
		}
		cols = append(cols, fieldName)
	}
	return cols
}
