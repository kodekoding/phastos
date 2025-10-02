package helper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/volatiletech/null"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/log"
)

func ConstructColNameAndValueBulk(ctx context.Context, arrayOfData interface{}, conditions ...map[string][]interface{}) (*database.CUDConstructData, error) {
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
	conditionSend := false
	if conditions != nil && len(conditions) > 0 {
		conditionSend = true
	}
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
			if conditionSend {
				for _, val := range conditions[0] {
					vals = append(vals, val[idx])
				}
			}
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

	result := generateBulk(columns, columnValues, arrayOfValues, conditions...)
	return result, nil
}

func generateBulk(columns []string, columnValues []interface{}, arrayOfValues [][]interface{}, condition ...map[string][]interface{}) *database.CUDConstructData {
	var listOfBulkValues []string

	result := new(database.CUDConstructData)
	result.ColsInsert = strings.Join(columns, ",")
	result.Values = columnValues
	if condition == nil {
		for _, values := range arrayOfValues {
			listOfBulkValues = append(listOfBulkValues, fmt.Sprintf("(?%s)", strings.Repeat(",?", len(values)-1)))
		}
		result.BulkValues = strings.Join(listOfBulkValues, ",")
	} else {
		var initJoinQuery strings.Builder
		var setAndWhereBulkQuery strings.Builder
		initJoinQuery.WriteString("SELECT ")
		setAndWhereBulkQuery.WriteString(" SET ")
		lenCols := len(columns)
		for x, colName := range columns {
			initJoinQuery.WriteString(fmt.Sprintf("? AS %s, ", colName))
			setAndWhereBulkQuery.WriteString(fmt.Sprintf("main_table.%s", colName))
			if x < lenCols-1 {
				setAndWhereBulkQuery.WriteString(", ")
			}
		}

		conditionLength := len(condition[0])
		setAndWhereBulkQuery.WriteString(" WHERE ")
		condCounter := 0
		for col := range condition[0] {
			initJoinQuery.WriteString(fmt.Sprintf("? AS %s, ", col))
			setAndWhereBulkQuery.WriteString(fmt.Sprintf("main_table.%s = join_table.%s", col, col))
			if condCounter < conditionLength-1 {
				setAndWhereBulkQuery.WriteString(" AND ")
			}
			condCounter++
		}

		initialBulkUpdateStr := initJoinQuery.String()
		initialBulkUpdateStr = initialBulkUpdateStr[:len(initialBulkUpdateStr)-2]
		listOfBulkValues = append(listOfBulkValues, initialBulkUpdateStr)
		initJoinQuery.Reset()
		for _, values := range arrayOfValues {
			listOfBulkValues = append(listOfBulkValues, fmt.Sprintf("SELECT ?%s", strings.Repeat(",?", len(values)-1)))
		}
		result.BulkValues = strings.Join(listOfBulkValues, " UNION ALL ")
		result.BulkQuery = setAndWhereBulkQuery.String()
	}

	return result
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

func readField(ctx context.Context, reflectVal reflect.Value, isNullStruct ...bool) ([]string, []interface{}) {
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
			if number, valid := value.(int); valid && number == 0 {
				continue
			} else if number64, valid := value.(int64); valid && number64 == 0 {
				continue
			} else if strVal, valid := value.(string); valid && strVal == "" {
				continue
			}
		}
		colTagVal, hasColTag := fieldType.Tag.Lookup("col")
		if hasColTag && colTagVal == "pk" {
			continue
		}
		fieldTypeData := fieldType.Type.String()
		nullStruct := strings.Contains(fieldTypeData, "null.") || colTagVal == "json"

		if nullStruct {
			containsNullStruct = true
		}

		if field.Kind() == reflect.Ptr {
			// to check nil pointer of data type
			if reflect.Indirect(field).Kind() == reflect.Invalid {
				continue
			}

			field = field.Elem()
		}

		switch field.Kind() {
		case reflect.Map:
			cols = append(cols, colName)
			values = append(values, value)
			continue
		case reflect.Struct:
			embeddedCols, embeddedVals := ConstructColNameAndValue(ctx, field.Interface(), containsNullStruct)

			if colTagVal == "json" && embeddedVals != nil {
				cols = append(cols, colName)
				values = append(values, value)
				continue
			}
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

		if fieldType.Name == "NullContent" {
			if value.(bool) {

				cols = append(cols, colName)
				values = append(values, "null")
			}
			continue
		}

		switch field.Kind() {
		case reflect.String:
			if str, valid := value.(string); valid && str == "null" {
				value = null.String{}
			} else if field.String() == "" {
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
	// change cols list with suffix '=?' using go routine
	columns := strings.Join(cols, ",")

	mutex := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	colLength := len(cols)
	haveUpdatedAtCol := false
	for i := 0; i < colLength; i++ {
		wg.Add(1)
		go func(col *string, wg *sync.WaitGroup, mtx *sync.Mutex) {
			mtx.Lock()
			if *col == "updated_at" {
				haveUpdatedAtCol = true
			}
			*col = *col + "=?"
			mtx.Unlock()
			wg.Done()
		}(&cols[i], wg, mutex)
	}

	wg.Wait()
	if !haveUpdatedAtCol {
		cols = append(cols, "updated_at=?")
		columns = fmt.Sprintf("%s,updated_at", columns)
		values = append(values, time.Now().Format("2006-01-02 15:04:05"))
	}

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

	var elem reflect.Value
	elem = reflectVal
	if elem.Kind() == reflect.Slice {
		// val is the slice
		typ := reflectVal.Type().Elem()
		if typ.Kind() == reflect.Ptr {
			elem = reflect.New(typ.Elem())
		}
		if typ.Kind() == reflect.Struct {
			elem = reflect.New(typ)
		}

		elem = elem.Elem()
	}

	refType := elem.Type()
	var cols []string

	var containsNullStruct bool
	if isNullStruct != nil {
		containsNullStruct = isNullStruct[0]
	}

	elemNumField := elem.NumField()
	for i := 0; i < elemNumField; i++ {
		field := elem.Field(i)

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
