package helper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	plog "github.com/kodekoding/phastos/v2/go/log"

	"github.com/volatiletech/null"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

// colTagJSON is now defined in struct_cache.go

// strNull is the string literal that represents SQL NULL.
const strNull = "null"

func ConstructColNameAndValueBulk(ctx context.Context, arrayOfData interface{}, conditions ...map[string][]interface{}) (*database.CUDConstructData, error) {
	reflectVal := reflect.ValueOf(arrayOfData)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Slice {
		return nil, errors.New("second parameter should be Slice/Array")
	}

	totalData := reflectVal.Len()

	maxLen := 0
	counterMaxLen := 0
	var columns []string
	var columnValues []interface{}
	var arrayOfValues [][]interface{}

	mapBulkValues := make(map[string][]interface{})
	conditionSend := len(conditions) > 0

	// Sequential loop is faster than goroutine+mutex for trivial operations.
	// The reflect + readField work per element is lightweight; goroutine
	// scheduling + mutex contention overhead exceeds the benefit for
	// typical bulk sizes (10-100 rows). Same pattern as checkKeyword()
	// and ConstructColNameAndValueForUpdate() optimizations.
	for i := 0; i < totalData; i++ {
		data := reflect.Indirect(reflectVal.Index(i))
		cols, vals := readField(ctx, data)
		if conditionSend {
			for _, val := range conditions[0] {
				vals = append(vals, val[i])
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
	}

	if counterMaxLen > 1 {
		return nil, errors.New("length of each element is different")
	}

	// Validate that all columns have the same number of values
	maxLen = 0
	counterMaxLen = 0
	for _, data := range mapBulkValues {
		jumlahData := len(data)
		if jumlahData != maxLen {
			counterMaxLen++
			maxLen = jumlahData
		}
		if counterMaxLen > 1 {
			return nil, errors.New("length of each field is different")
		}
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
	typeInfo := getStructTypeInfo(reflectVal.Type())
	numField := typeInfo.NumField

	// Pre-allocate slices with capacity hint (B5)
	cols := make([]string, 0, numField)
	values := make([]interface{}, 0, numField)

	var containsNullStruct bool
	if isNullStruct != nil {
		containsNullStruct = isNullStruct[0]
	}

	for i := 0; i < numField; i++ {
		fi := &typeInfo.Fields[i]
		field := reflectVal.Field(fi.Index)
		value := field.Interface()

		// Skip fields with `db:"-"` tag
		if fi.ColName == "-" {
			continue
		}

		// Skip zero-value id fields
		if fi.SkipZero {
			if number, valid := value.(int); valid && number == 0 {
				continue
			} else if number64, valid := value.(int64); valid && number64 == 0 {
				continue
			} else if strVal, valid := value.(string); valid && strVal == "" {
				continue
			}
		}

		// Skip primary key fields
		if fi.IsPk {
			continue
		}

		nullStruct := fi.IsNullType
		if nullStruct {
			containsNullStruct = true
		}

		if fi.IsPtr {
			// to check nil pointer of data type
			if reflect.Indirect(field).Kind() == reflect.Invalid {
				continue
			}
			field = field.Elem()
		}

		switch field.Kind() {
		case reflect.Map:
			cols = append(cols, fi.ColName)
			values = append(values, value)
			continue
		case reflect.Struct:
			embeddedCols, embeddedVals := ConstructColNameAndValue(ctx, field.Interface(), containsNullStruct)

			if fi.IsJSONCol && embeddedVals != nil {
				cols = append(cols, fi.ColName)
				values = append(values, value)
				continue
			}
			if nullStruct && embeddedVals != nil {
				cols = append(cols, fi.ColName)
			} else {
				cols = append(cols, embeddedCols...)
			}
			values = append(values, embeddedVals...)

			continue
		}

		if fi.IsValid {
			if !(value.(bool)) { //nolint:errcheck

				cols = nil
				values = nil
			}
			continue
		}

		if fi.IsNullCont {
			if value.(bool) { //nolint:errcheck

				cols = append(cols, fi.ColName)
				values = append(values, "null")
			}
			continue
		}

		if fi.IsString {
			if str, valid := value.(string); valid && str == strNull { //nolint:errcheck
				value = null.String{} //nolint:errcheck
			} else if field.String() == "" {
				continue
			}
		}

		cols = append(cols, fi.ColName)
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

	reflectVal := reflect.ValueOf(structName)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	// Use cached update template — skips readField + Remove loop entirely.
	// Template is computed once per struct type and cached in struct_cache.go.
	tmpl := GetUpdateTemplate(reflectVal.Type())
	info := ExtractUpdateValues(tmpl, reflectVal, anotherValues...)

	// Build ColsInsert from the actual Cols (which may have =null entries)
	var colsInsertBuilder strings.Builder
	for i, col := range info.Cols {
		if i > 0 {
			colsInsertBuilder.WriteByte(',')
		}
		if idx := strings.Index(col, "="); idx >= 0 {
			colsInsertBuilder.WriteString(col[:idx])
		} else {
			colsInsertBuilder.WriteString(col)
		}
	}

	result := database.GetCUDConstructData()
	result.Cols = info.Cols
	result.ColsInsert = colsInsertBuilder.String()
	result.Values = info.Values
	return result
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
	includedColsNotNull := optionalParams.includedCols != ""
	excludedColsNotNull := optionalParams.excludedCols != ""

	if includedColsNotNull && excludedColsNotNull {
		return nil
	}

	// B2: Check cache for this type + column filter combination.
	// Column lists are invariant per struct type, so caching eliminates
	// repeated reflection + ToSnakeCase calls on every GetList().
	cacheKey := selectCacheKey{
		Type:     refType,
		Excluded: optionalParams.excludedCols,
		Included: optionalParams.includedCols,
	}
	if cached, ok := getSelectColsFromCache(cacheKey); ok {
		return cached
	}

	var cols []string
	typeInfo := getStructTypeInfo(refType)

	for i := 0; i < typeInfo.NumField; i++ {
		fi := &typeInfo.Fields[i]
		field := elem.Field(fi.Index)
		value := field.Interface()

		if fi.IsNullType {
			populateColumns(includedColsNotNull, excludedColsNotNull, &cols, fi.ColName, optionalParams)
			continue
		}
		if fi.IsStruct {
			opts = append(opts, WithIsEmbeddedStruct(true))
			embeddedCols := GenerateSelectCols(ctx, value, opts...)
			cols = append(cols, embeddedCols...)
			continue
		}

		populateColumns(includedColsNotNull, excludedColsNotNull, &cols, fi.ColName, optionalParams)
	}

	putSelectColsToCache(cacheKey, cols)
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
