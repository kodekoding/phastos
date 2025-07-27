package helper

import (
	"context"
	"errors"
	"fmt"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/volatiletech/null"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/monitoring"
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
	log := plog.Ctx(ctx)
	if isNullStruct == nil {
		txn := monitoring.BeginTrxFromContext(ctx)
		if txn != nil {
			defer txn.StartSegment("StructHelper-ConstructColNameAndValue").End()
		}
	}
	reflectVal := reflect.ValueOf(structName)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Struct {
		log.Error().Msg("second parameter should be struct")
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

func ConvertStructToMap(structSource interface{}) map[string]interface{} {
	reflectVal := reflect.ValueOf(structSource)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Struct {
		return nil
	}
	refType := reflectVal.Type()

	totalField := reflectVal.NumField()
	result := make(map[string]interface{})
	for i := 0; i < totalField; i++ {
		field := reflectVal.Field(i)
		fieldType := refType.Field(i)
		colName := fieldType.Tag.Get("csv")
		if colName == "" {
			colName = fieldType.Tag.Get("excel")
		}
		if colName == "" {
			// set default colName with field name
			colName = fieldType.Name
		}
		colDataType := fieldType.Type.String()
		//log.Println(colName, " has type data: ", colDataType)
		switch colDataType {
		case "int", "int64", "int16", "int8", "int32":
			field.SetInt(0)
		default:
			// all data type except "int" will be force to string
			field.SetString("")
		}
		result[colName] = field.Interface()

	}

	return result
}

func ConstructColNameAndValueForUpdate(ctx context.Context, structName interface{}, anotherValues ...interface{}) *database.CUDConstructData {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		defer txn.StartSegment("StructHelper-ConstructColNameAndValueForUpdate").End()
	}
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "Helper-ConstructColNameAndValueForUpdate")
	//defer trc.Finish()
	cols, values := ConstructColNameAndValue(ctx, structName)
	// change cols list with suffix '=?' using go routine
	columns := strings.Join(cols, ",")

	mutex := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	colLength := len(cols)
	haveUpdatedAtCol := false
	for i := 0; i < colLength; i++ {
		wg.Add(1)
		go func(index int, col *string, vals interface{}, wg *sync.WaitGroup, mtx *sync.Mutex) {
			mtx.Lock()
			if *col == "updated_at" {
				haveUpdatedAtCol = true
			}

			_, valid := vals.(null.String)
			if vals == nil || valid {
				*col = *col + "=null"
				values = Remove(values, index)
			} else {
				*col = *col + "=?"
			}
			mtx.Unlock()
			wg.Done()
		}(i, &cols[i], values[i], wg, mutex)
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

type GenSelectColsOptions func(*GenSelectColsOptionalParams)
type GenSelectColsOptionalParams struct {
	isEmbeddedStruct bool
	excludedCols     string
	includedCols     string
}

func WithIsEmbeddedStruct(isEmbeddedStruct bool) GenSelectColsOptions {
	return func(params *GenSelectColsOptionalParams) {
		params.isEmbeddedStruct = isEmbeddedStruct
	}
}

func WithExcludedCols(excludedCols string) GenSelectColsOptions {
	return func(params *GenSelectColsOptionalParams) {
		params.excludedCols = excludedCols
	}
}
func WithIncludedCols(includedCols string) GenSelectColsOptions {
	return func(params *GenSelectColsOptionalParams) {
		params.includedCols = includedCols
	}
}

func GenerateSelectCols(ctx context.Context, source interface{}, opts ...GenSelectColsOptions) []string {
	optionalParams := new(GenSelectColsOptionalParams)
	for _, opt := range opts {
		opt(optionalParams)
	}
	if !optionalParams.isEmbeddedStruct {
		txn := monitoring.BeginTrxFromContext(ctx)
		if txn != nil {
			defer txn.StartSegment("StructHelper-GenerateSelectCols").End()
		}
	}

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

	includedColsNotNull := optionalParams.includedCols != ""
	excludedColsNotNull := optionalParams.excludedCols != ""

	if includedColsNotNull && excludedColsNotNull {
		return nil
	}

	elemNumField := elem.NumField()

	wg := new(sync.WaitGroup)
	mtx := new(sync.Mutex)
	for i := 0; i < elemNumField; i++ {
		wg.Add(1)
		go func(index int, element reflect.Value, columns *[]string, wait *sync.WaitGroup, mute *sync.Mutex) {
			mute.Lock()
			defer func() {
				wait.Done()
				mute.Unlock()
			}()
			field := elem.Field(index)

			value := field.Interface()
			fieldType := refType.Field(index)
			fieldName := ToSnakeCase(fieldType.Name)
			val, exist := fieldType.Tag.Lookup("db")
			if exist {
				fieldName = val
			}
			fieldTypeData := fieldType.Type.String()
			colTagVal, hasColTag := fieldType.Tag.Lookup("col")

			if strings.Contains(fieldTypeData, "null.") || (hasColTag && colTagVal == "json") {
				populateColumns(includedColsNotNull, excludedColsNotNull, columns, fieldName, optionalParams)
				return
			}
			if field.Kind() == reflect.Struct {
				opts = append(opts, WithIsEmbeddedStruct(true))
				embeddedCols := GenerateSelectCols(ctx, value, opts...)
				*columns = append(*columns, embeddedCols...)
				return
			}

			populateColumns(includedColsNotNull, excludedColsNotNull, columns, fieldName, optionalParams)

		}(i, elem, &cols, wg, mtx)
	}

	wg.Wait()
	return cols
}

func populateColumns(includedColsNotNull bool, excludedColsNotNull bool, columns *[]string, fieldName string, optionalParams *GenSelectColsOptionalParams) {
	if !includedColsNotNull && !excludedColsNotNull {
		// condition when include + exclude cols is ""
		*columns = append(*columns, fieldName)
	} else if includedColsNotNull && strings.Contains(optionalParams.includedCols, fieldName) {
		// condition when field name is registered on included cols string
		*columns = append(*columns, fieldName)
	} else if excludedColsNotNull && !strings.Contains(optionalParams.excludedCols, fieldName) {
		*columns = append(*columns, fieldName)
	}
}
