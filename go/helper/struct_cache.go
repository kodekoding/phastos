package helper

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/volatiletech/null"
)

const (
	colTagDB   = "db"
	colTagCol  = "col"
	colTagJSON = "json"

	// timeFormatLayout is the Go reference time layout used for formatting
	// timestamps in UPDATE queries. Pre-computed as a constant to avoid
	// repeated string literal allocation.
	timeFormatLayout = "2006-01-02 15:04:05"
)

// structFieldInfo holds pre-computed metadata for a single struct field.
// This is computed once per reflect.Type and cached, eliminating repeated
// Tag.Get/Lookup and Type.String() calls on every query.
type structFieldInfo struct {
	Index       int
	ColName     string // from `db` tag or snake_case of field name
	IsStruct    bool
	IsPtr       bool
	IsNullType  bool   // field type contains "null."
	IsJSONCol   bool   // `col` tag == "json"
	ColTagVal   string // raw value of `col` tag
	SkipZero    bool   // colName == "id" — skip if zero value
	IsPk        bool   // `col` tag == "pk"
	IsValid     bool   // field type name == "Valid"
	IsNullCont  bool   // field type name == "NullContent"
	IsMap       bool
	IsString    bool
	FieldTypeNm string // e.g. "null.String", "int64", etc.
}

// structTypeInfo holds the cached field metadata for a given reflect.Type.
type structTypeInfo struct {
	Fields   []structFieldInfo
	NumField int
	ColNames []string // pre-computed list of all non-skipped column names
}

// structCache caches structTypeInfo by reflect.Type.
// Key: reflect.Type (pointer), Value: *structTypeInfo
var structCache sync.Map

// getStructTypeInfo returns cached struct field info, computing it on first access.
func getStructTypeInfo(t reflect.Type) *structTypeInfo {
	if cached, ok := structCache.Load(t); ok {
		return cached.(*structTypeInfo) //nolint:errcheck
	}

	info := &structTypeInfo{
		NumField: t.NumField(),
	}

	// Pre-allocate fields slice
	fields := make([]structFieldInfo, 0, info.NumField)
	var colNames []string

	for i := 0; i < info.NumField; i++ {
		fieldType := t.Field(i)
		f := structFieldInfo{
			Index:       i,
			FieldTypeNm: fieldType.Type.String(),
		}

		// Resolve column name from `db` tag
		dbTag, hasDB := fieldType.Tag.Lookup(colTagDB)
		if hasDB {
			f.ColName = dbTag
		} else {
			f.ColName = ToSnakeCase(fieldType.Name)
		}

		// Resolve `col` tag
		colTagVal, hasColTag := fieldType.Tag.Lookup(colTagCol)
		f.ColTagVal = colTagVal
		f.IsJSONCol = hasColTag && colTagVal == colTagJSON
		f.IsPk = hasColTag && colTagVal == "pk"
		f.IsValid = fieldType.Name == "Valid"
		f.IsNullCont = fieldType.Name == "NullContent"
		f.IsNullType = strings.Contains(f.FieldTypeNm, "null.") || f.IsJSONCol
		f.SkipZero = f.ColName == "id"
		f.IsPtr = fieldType.Type.Kind() == reflect.Ptr

		kind := fieldType.Type.Kind()
		f.IsStruct = kind == reflect.Struct && !f.IsNullType && !f.IsValid && !f.IsNullCont
		f.IsMap = kind == reflect.Map
		f.IsString = kind == reflect.String

		fields = append(fields, f)

		// Track non-skipped column names for quick access
		if f.ColName != "-" && !f.IsPk && !f.IsValid && !f.IsNullCont {
			colNames = append(colNames, f.ColName)
		}
	}

	info.Fields = fields
	info.ColNames = colNames

	// Store in cache (using LoadOrStore to handle concurrent first-access)
	actual, _ := structCache.LoadOrStore(t, info)
	return actual.(*structTypeInfo) //nolint:errcheck
}

// selectColsCache caches GenerateSelectCols results.
// Key: selectCacheKey{Type, Excluded, Included}, Value: []string
var selectColsCache sync.Map

type selectCacheKey struct {
	Type     reflect.Type
	Excluded string
	Included string
}

// getSelectColsFromCache returns cached select columns if available.
func getSelectColsFromCache(key selectCacheKey) ([]string, bool) {
	val, ok := selectColsCache.Load(key)
	if !ok {
		return nil, false
	}
	return val.([]string), true //nolint:errcheck
}

// putSelectColsToCache stores computed select columns in cache.
func putSelectColsToCache(key selectCacheKey, cols []string) {
	// Store a copy to avoid mutations from callers
	cached := make([]string, len(cols))
	copy(cached, cols)
	selectColsCache.Store(key, cached)
}

// =============================================================
// Update template cache — eliminates repeated reflection + string
// building for UpdateById/Update operations where the struct type
// determines a fixed query template.
// =============================================================

// UpdateFieldPath describes how to extract a single value from a struct
// for an UPDATE SET clause. It handles embedded structs via FieldByIndex.
type UpdateFieldPath struct {
	// IndexPath is used with reflect.Value.FieldByIndex to reach
	// the leaf field (e.g. [0] for a top-level field, [2,1] for
	// an embedded struct's field).
	IndexPath  []int
	ColName    string // e.g. "name", "age", "updated_at"
	IsNullType bool   // true if the field type contains "null." — treat nil/null specially
}

// UpdateTemplateInfo holds the pre-computed UPDATE query template for a
// given struct type. On each call, only the values need to be extracted
// from the struct instance; the column layout is fixed.
type UpdateTemplateInfo struct {
	// Cols is the list of SET clause fragments like ["name=?", "age=?", "updated_at=?"]
	// This is what data.Cols will be set to.
	Cols []string
	// ColsInsert is the comma-joined column names like "name,age,updated_at"
	ColsInsert string
	// FieldPaths tells us how to extract each non-null value from the struct.
	// The i-th path corresponds to the i-th "?" placeholder in Cols.
	FieldPaths []UpdateFieldPath
	// HaveUpdatedAt is true when the struct itself has an updated_at field.
	HaveUpdatedAt bool
}

// updateTemplateCache caches updateTemplateInfo per reflect.Type.
// The template is the same regardless of the actual struct instance
// because the column layout is determined entirely by the type.
var updateTemplateCache sync.Map

// GetUpdateTemplate returns the cached update template for the given struct type,
// computing it on first access. The template assumes all non-null fields are
// present (the common case). Fields with null.String values that are valid
// (i.e. non-NULL in SQL) are included; null values are excluded at runtime.
func GetUpdateTemplate(t reflect.Type) *UpdateTemplateInfo {
	if cached, ok := updateTemplateCache.Load(t); ok {
		return cached.(*UpdateTemplateInfo) //nolint:errcheck
	}

	info := computeUpdateTemplate(t)
	actual, _ := updateTemplateCache.LoadOrStore(t, info)
	return actual.(*UpdateTemplateInfo) //nolint:errcheck
}

// computeUpdateTemplate builds the update template by walking all fields
// using the cached struct type info.
func computeUpdateTemplate(t reflect.Type) *UpdateTemplateInfo {
	typeInfo := getStructTypeInfo(t)
	var (
		cols       []string
		fieldPaths []UpdateFieldPath
	)
	haveUpdatedAt := false

	for i := 0; i < typeInfo.NumField; i++ {
		fi := &typeInfo.Fields[i]

		// Skip fields that never appear in UPDATE SET
		// created_at is set once at INSERT and never updated.
		if fi.ColName == "-" || fi.IsPk || fi.IsValid || fi.IsNullCont || fi.ColName == "created_at" {
			continue
		}

		// Skip zero-value id fields (they're never updated)
		if fi.SkipZero {
			continue
		}

		// Skip maps — not updatable via ORM
		if fi.IsMap {
			continue
		}

		if fi.ColName == "updated_at" {
			haveUpdatedAt = true
		}

		// Handle embedded structs — recurse into them
		if fi.IsStruct {
			embeddedType := t.Field(i).Type
			embeddedTemplate := computeUpdateTemplate(embeddedType)
			for _, ep := range embeddedTemplate.FieldPaths {
				// Prefix the embedded struct's field index path
				path := make([]int, 0, len(ep.IndexPath)+1)
				path = append(path, fi.Index)
				path = append(path, ep.IndexPath...)
				cols = append(cols, ep.ColName+"=?")
				fieldPaths = append(fieldPaths, UpdateFieldPath{
					IndexPath:  path,
					ColName:    ep.ColName,
					IsNullType: ep.IsNullType,
				})
			}
			if !haveUpdatedAt && embeddedTemplate.HaveUpdatedAt {
				haveUpdatedAt = true
			}
			continue
		}

		// Regular field — add to template
		cols = append(cols, fi.ColName+"=?")
		fieldPaths = append(fieldPaths, UpdateFieldPath{
			IndexPath:  []int{fi.Index},
			ColName:    fi.ColName,
			IsNullType: fi.IsNullType,
		})
	}

	// If struct doesn't have updated_at, we add it automatically
	if !haveUpdatedAt {
		cols = append(cols, "updated_at=?")
	}

	colsInsert := strings.Join(func() []string {
		names := make([]string, len(cols))
		for i, c := range cols {
			// Strip the "=?" or "=null" suffix to get just the column name
			if idx := strings.Index(c, "="); idx >= 0 {
				names[i] = c[:idx]
			} else {
				names[i] = c
			}
		}
		return names
	}(), ",")

	return &UpdateTemplateInfo{
		Cols:          cols,
		ColsInsert:    colsInsert,
		FieldPaths:    fieldPaths,
		HaveUpdatedAt: haveUpdatedAt,
	}
}

// ExtractUpdateValues uses the cached field paths to extract values from
// a struct instance. It returns the Cols pattern and Values for the UPDATE.
// Null fields (nil pointers, null.String with Valid=false) are excluded.
func ExtractUpdateValues(tmpl *UpdateTemplateInfo, structVal reflect.Value, anotherValues ...interface{}) *CUDConstructInfo {
	result := &CUDConstructInfo{
		HaveUpdatedAt: tmpl.HaveUpdatedAt,
	}

	// Pre-allocate with capacity for all fields + possible anotherValues
	maxVals := len(tmpl.FieldPaths) + 1 + len(anotherValues) // +1 for updated_at
	result.Values = make([]interface{}, 0, maxVals)
	result.Cols = make([]string, 0, len(tmpl.Cols))

	for _, fp := range tmpl.FieldPaths {
		field := structVal.FieldByIndex(fp.IndexPath)
		value := field.Interface()

		// Handle null.String — skip if not Valid (SQL NULL)
		if fp.IsNullType {
			if ns, ok := value.(null.String); ok {
				if !ns.Valid {
					// This field is NULL — add "=null" and skip value
					result.Cols = append(result.Cols, fp.ColName+"=null")
					continue
				}
			}
		}

		// Handle nil pointers — skip
		if field.Kind() == reflect.Ptr && field.IsNil() {
			result.Cols = append(result.Cols, fp.ColName+"=null")
			continue
		}

		// Handle empty strings — skip
		if field.Kind() == reflect.String && field.String() == "" {
			continue
		}

		// Handle "null" string literal
		if str, ok := value.(string); ok && str == strNull { //nolint:errcheck
			value = null.String{} //nolint:errcheck
		}

		result.Cols = append(result.Cols, fp.ColName+"=?")
		result.Values = append(result.Values, value)
	}

	// Add updated_at if struct doesn't have it
	if !tmpl.HaveUpdatedAt {
		result.Cols = append(result.Cols, "updated_at=?")
		result.Values = append(result.Values, time.Now().Format(timeFormatLayout))
	}

	if len(anotherValues) > 0 {
		result.Values = append(result.Values, anotherValues...)
	}

	return result
}

// CUDConstructInfo is the result of extracting update values from a struct
// using the cached template. It contains the same data as CUDConstructData
// but is produced without repeated reflection/tag lookups.
type CUDConstructInfo struct {
	Cols          []string
	Values        []interface{}
	HaveUpdatedAt bool
}

// ExtractFixedUpdateValues extracts values from a struct using the FULL
// template — every field is included regardless of null/empty status.
// For null.String with Valid=false, it sends nil (SQL NULL).
// For nil pointers, it sends nil (SQL NULL).
// For empty strings, it sends "" (empty string) to avoid violating NOT NULL
// constraints on columns like email, name, etc.
//
// This produces an INVARIANT query string (same SET clause every time)
// which means the same *sql.Stmt can be reused across calls — critical
// for PostgreSQL prepared statement caching where variable-length SET
// clauses cause a new prepare on every call.
//
// The Values slice length always matches len(tmpl.FieldPaths) + [1 if !HaveUpdatedAt] + len(anotherValues).
func ExtractFixedUpdateValues(tmpl *UpdateTemplateInfo, structVal reflect.Value, anotherValues ...interface{}) *CUDConstructInfo {
	numVals := len(tmpl.FieldPaths)
	if !tmpl.HaveUpdatedAt {
		numVals++
	}
	numVals += len(anotherValues)

	values := make([]interface{}, 0, numVals)

	for _, fp := range tmpl.FieldPaths {
		field := structVal.FieldByIndex(fp.IndexPath)

		if fp.IsNullType {
			if ns, ok := field.Interface().(null.String); ok {
				if ns.Valid {
					values = append(values, ns.String)
				} else {
					values = append(values, nil)
				}
				continue
			}
		}

		// Handle nil pointers → SQL NULL
		if field.Kind() == reflect.Ptr && field.IsNil() {
			values = append(values, nil)
			continue
		}

		// Handle empty strings — send empty string (not NULL) to avoid
		// violating NOT NULL constraints on columns like email.
		// The original dynamic path skipped these fields entirely, but
		// the fixed template must include every field.
		if field.Kind() == reflect.String && field.String() == "" {
			values = append(values, "")
			continue
		}

		value := field.Interface()
		// Handle "null" string literal
		if str, ok := value.(string); ok && str == strNull { //nolint:errcheck
			value = nil
		}

		values = append(values, value)
	}

	// Add updated_at if struct doesn't have it
	if !tmpl.HaveUpdatedAt {
		values = append(values, time.Now().Format(timeFormatLayout))
	}

	if len(anotherValues) > 0 {
		values = append(values, anotherValues...)
	}

	return &CUDConstructInfo{
		Cols:          tmpl.Cols,
		Values:        values,
		HaveUpdatedAt: tmpl.HaveUpdatedAt,
	}
}
