package helper

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/volatiletech/null"
)

// Test structs for struct helper functions
type testUser struct {
	Id        int          `db:"id"`
	Name      string       `db:"name"`
	Email     string       `db:"email"`
	Age       int          `db:"age"`
	CreatedAt string       `db:"created_at"`
	UpdatedAt null.String  `db:"updated_at"`
}

type testUserWithCol struct {
	Id    int    `db:"id" col:"pk"`
	Name  string `db:"name"`
	Email string `db:"email" col:"json"`
}

type testUserWithIgnore struct {
	Id       int    `db:"id"`
	Name     string `db:"name"`
	Password string `db:"-"`
}

type testEmbeddedUser struct {
	testUser
	Role string `db:"role"`
}

type simpleItem struct {
	Id    int    `db:"id"`
	Title string `db:"title"`
	Price int64  `db:"price"`
}

type nullableItem struct {
	Id          int          `db:"id"`
	Name        string       `db:"name"`
	Description null.String  `db:"description"`
}

// ---- ConstructColNameAndValue ----

func TestConstructColNameAndValue(t *testing.T) {
	t.Run("should extract columns and values from simple struct", func(t *testing.T) {
		user := testUser{
			Id:        1,
			Name:      "John",
			Email:     "john@example.com",
			Age:       30,
			CreatedAt: "2024-01-01",
			UpdatedAt: null.StringFrom("2024-01-02"),
		}

		cols, values := ConstructColNameAndValue(context.Background(), user)
		assert.NotNil(t, cols)
		assert.NotNil(t, values)

		// id should be skipped (SkipZero for id==0 but this id=1 should also be skipped due to IsPk)
		// Actually id=1 is skipped because colName=="id" triggers IsPk check
		// The `id` field has col:"" so colName is "id" from db tag, and since colName=="id" it's IsPk=false
		// Wait: IsPk is set when `col` tag == "pk". For testUser, Id has no col tag, so IsPk is false.
		// But SkipZero is true for colName=="id", and Id=1 (not zero), so it should be included
		// Actually SkipZero skips when value is 0. Id=1 is not zero, so it's NOT skipped.
		// But then IsPk check: IsPk = hasColTag && colTagVal == "pk". No col tag on testUser.Id, so IsPk=false.
		// So Id=1 should appear... but the primary key skip: "id" field is SkipZero=true, but value=1 != 0, so not skipped.
		// The result should include: name, email, age, created_at, updated_at (id=1 is kept since non-zero)
		// Actually wait - skipZero only skips when value IS zero. Since id=1, it should be included.
		assert.Contains(t, cols, "name")
		assert.Contains(t, cols, "email")
	})

	t.Run("should skip zero-value id field", func(t *testing.T) {
		user := testUser{
			Id:    0,
			Name:  "John",
			Email: "john@example.com",
			Age:   30,
		}

		cols, _ := ConstructColNameAndValue(context.Background(), user)
		assert.NotNil(t, cols)
		// id=0 should be skipped due to SkipZero
		assert.NotContains(t, cols, "id")
	})

	t.Run("should skip fields with db:\"-\" tag", func(t *testing.T) {
		user := testUserWithIgnore{
			Id:       1,
			Name:     "John",
			Password: "secret",
		}

		cols, _ := ConstructColNameAndValue(context.Background(), user)
		assert.NotNil(t, cols)
		assert.NotContains(t, cols, "-")
		assert.NotContains(t, cols, "Password")
		assert.Contains(t, cols, "name")
	})

	t.Run("should skip pk fields", func(t *testing.T) {
		user := testUserWithCol{
			Id:    1,
			Name:  "John",
			Email: `{"work":"john@work.com"}`,
		}

		cols, _ := ConstructColNameAndValue(context.Background(), user)
		assert.NotNil(t, cols)
		// Id has col:"pk" so it's marked as pk and should be skipped
		assert.NotContains(t, cols, "id")
	})

	t.Run("should return nil for non-struct input", func(t *testing.T) {
		cols, values := ConstructColNameAndValue(context.Background(), "not a struct")
		assert.Nil(t, cols)
		assert.Nil(t, values)
	})

	t.Run("should handle pointer to struct", func(t *testing.T) {
		user := &testUser{
			Id:   0,
			Name: "Jane",
		}

		cols, _ := ConstructColNameAndValue(context.Background(), user)
		assert.NotNil(t, cols)
		assert.Contains(t, cols, "name")
	})

	t.Run("should skip empty string fields", func(t *testing.T) {
		user := testUser{
			Id:   0,
			Name: "",
		}

		cols, _ := ConstructColNameAndValue(context.Background(), user)
		// Empty string fields are skipped (IsString check)
		assert.NotContains(t, cols, "name")
	})
}

// ---- ConstructColNameAndValueForUpdate ----

func TestConstructColNameAndValueForUpdate(t *testing.T) {
	t.Run("should generate update construct for simple struct", func(t *testing.T) {
		user := testUser{
			Id:        1,
			Name:      "John",
			Email:     "john@example.com",
			Age:       30,
			CreatedAt: "2024-01-01",
			UpdatedAt: null.StringFrom("2024-01-02"),
		}

		result := ConstructColNameAndValueForUpdate(context.Background(), user)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Cols)
		assert.NotEmpty(t, result.Values)
		assert.NotEmpty(t, result.ColsInsert)

		// id should be skipped (SkipZero) and created_at should be skipped in update
		for _, col := range result.Cols {
			assert.NotContains(t, col, "created_at")
		}
	})

	t.Run("should add updated_at automatically if struct doesn't have it", func(t *testing.T) {
		item := simpleItem{
			Id:    1,
			Title: "Test",
			Price: 100,
		}

		result := ConstructColNameAndValueForUpdate(context.Background(), item)
		assert.NotNil(t, result)

		// Should have updated_at since simpleItem doesn't have one
		hasUpdatedAt := false
		for _, col := range result.Cols {
			if col == "updated_at=?" {
				hasUpdatedAt = true
			}
		}
		assert.True(t, hasUpdatedAt, "expected updated_at to be added automatically")
	})

	t.Run("should generate ColsInsert from Cols", func(t *testing.T) {
		user := testUser{
			Id:        0,
			Name:      "John",
			Email:     "john@example.com",
			Age:       30,
			CreatedAt: "2024-01-01",
			UpdatedAt: null.StringFrom("2024-01-02"),
		}

		result := ConstructColNameAndValueForUpdate(context.Background(), user)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ColsInsert)

		// ColsInsert should not contain "=?" or "=null"
		assert.NotContains(t, result.ColsInsert, "=?")
		assert.NotContains(t, result.ColsInsert, "=null")
	})

	t.Run("should handle null.String with Valid=false in update", func(t *testing.T) {
		item := nullableItem{
			Id:          1,
			Name:        "Test",
			Description: null.String{}, // Valid=false
		}

		result := ConstructColNameAndValueForUpdate(context.Background(), item)
		assert.NotNil(t, result)

		// description with Valid=false should NOT appear
		for _, col := range result.Cols {
			assert.NotContains(t, col, "description", "description should be skipped")
		}
	})

	t.Run("should append anotherValues", func(t *testing.T) {
		item := simpleItem{
			Id:    0,
			Title: "Test",
			Price: 100,
		}

		result := ConstructColNameAndValueForUpdate(context.Background(), item, 42)
		assert.NotNil(t, result)
		assert.Contains(t, result.Values, 42)
	})
}

// ---- ConstructColNameAndValueBulk ----

func TestConstructColNameAndValueBulk(t *testing.T) {
	t.Run("should generate bulk insert data from slice", func(t *testing.T) {
		items := []simpleItem{
			{Id: 0, Title: "Item1", Price: 10},
			{Id: 0, Title: "Item2", Price: 20},
			{Id: 0, Title: "Item3", Price: 30},
		}

		result, err := ConstructColNameAndValueBulk(context.Background(), items)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ColsInsert)
		assert.NotEmpty(t, result.Values)
		assert.NotEmpty(t, result.BulkValues)
	})

	t.Run("should return error for non-slice input", func(t *testing.T) {
		_, err := ConstructColNameAndValueBulk(context.Background(), "not a slice")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Slice/Array")
	})

	t.Run("should return error for pointer to non-slice", func(t *testing.T) {
		notSlice := "hello"
		_, err := ConstructColNameAndValueBulk(context.Background(), &notSlice)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Slice/Array")
	})

	t.Run("should handle empty slice", func(t *testing.T) {
		items := []simpleItem{}
		result, err := ConstructColNameAndValueBulk(context.Background(), items)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("should handle single element slice", func(t *testing.T) {
		items := []simpleItem{
			{Id: 0, Title: "Only", Price: 99},
		}

		result, err := ConstructColNameAndValueBulk(context.Background(), items)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.ColsInsert, "title")
	})

	t.Run("should handle pointer slice input", func(t *testing.T) {
		items := []*simpleItem{
			{Id: 0, Title: "Item1", Price: 10},
			{Id: 0, Title: "Item2", Price: 20},
		}

		result, err := ConstructColNameAndValueBulk(context.Background(), items)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// ---- GenerateSelectCols ----

func TestGenerateSelectCols(t *testing.T) {
	t.Run("should generate column names for simple struct", func(t *testing.T) {
		user := testUser{}
		cols := GenerateSelectCols(context.Background(), user)
		assert.NotEmpty(t, cols)
		assert.Contains(t, cols, "name")
		assert.Contains(t, cols, "email")
		assert.Contains(t, cols, "age")
	})

	t.Run("should generate columns from pointer to struct", func(t *testing.T) {
		user := &testUser{}
		cols := GenerateSelectCols(context.Background(), user)
		assert.NotEmpty(t, cols)
		assert.Contains(t, cols, "name")
	})

	t.Run("should generate columns from slice of struct", func(t *testing.T) {
		users := []testUser{}
		cols := GenerateSelectCols(context.Background(), users)
		assert.NotEmpty(t, cols)
	})

	t.Run("should include id column", func(t *testing.T) {
		user := testUser{}
		cols := GenerateSelectCols(context.Background(), user)
		assert.Contains(t, cols, "id")
	})

	t.Run("should exclude specified columns", func(t *testing.T) {
		user := testUser{}
		cols := GenerateSelectCols(context.Background(), user, WithExcludedCols("created_at,updated_at"))
		assert.NotContains(t, cols, "created_at")
		assert.NotContains(t, cols, "updated_at")
		assert.Contains(t, cols, "name")
	})

	t.Run("should include only specified columns", func(t *testing.T) {
		user := testUser{}
		cols := GenerateSelectCols(context.Background(), user, WithIncludedCols("name,email"))
		// Should only have name and email
		for _, col := range cols {
			assert.True(t, col == "name" || col == "email", "unexpected column: %s", col)
		}
		assert.Contains(t, cols, "name")
		assert.Contains(t, cols, "email")
	})

	t.Run("should return nil when both included and excluded are set", func(t *testing.T) {
		user := testUser{}
		cols := GenerateSelectCols(context.Background(), user, WithIncludedCols("name"), WithExcludedCols("email"))
		assert.Nil(t, cols)
	})

	t.Run("should handle fields with db:\"-\" tag", func(t *testing.T) {
		user := testUserWithIgnore{}
		cols := GenerateSelectCols(context.Background(), user)
		// Note: GenerateSelectCols includes columns with db:"-" in its output
		// because the filtering for "-" happens in readField, not in GenerateSelectCols
		assert.Contains(t, cols, "id")
		assert.Contains(t, cols, "name")
	})
}

// ---- ConvertStructToMap ----

func TestConvertStructToMap(t *testing.T) {
	t.Run("should convert struct to map using csv tags", func(t *testing.T) {
		type csvItem struct {
			Name  string `csv:"name"`
			Email string `csv:"email_address"`
			Age   int    `csv:"age"`
		}

		item := csvItem{Name: "John", Email: "john@test.com", Age: 30}
		// ConvertStructToMap modifies fields via reflection, so pass pointer
		result := ConvertStructToMap(&item)
		assert.NotNil(t, result)

		// After conversion, int fields are zeroed and string fields are emptied
		// The function modifies the field values before putting into map
		assert.Contains(t, result, "name")
		assert.Contains(t, result, "email_address")
		assert.Contains(t, result, "age")
	})

	t.Run("should use field name when no csv or excel tag", func(t *testing.T) {
		type plainItem struct {
			Name string
			Age  int
		}

		item := plainItem{Name: "John", Age: 30}
		result := ConvertStructToMap(&item)
		assert.NotNil(t, result)
		assert.Contains(t, result, "Name")
		assert.Contains(t, result, "Age")
	})

	t.Run("should return nil for non-struct input", func(t *testing.T) {
		result := ConvertStructToMap("not a struct")
		assert.Nil(t, result)
	})

	t.Run("should return nil for nil input", func(t *testing.T) {
		result := ConvertStructToMap(nil)
		assert.Nil(t, result)
	})

	t.Run("should handle pointer to struct", func(t *testing.T) {
		type simpleItem struct {
			Name string `csv:"name"`
		}
		item := &simpleItem{Name: "Test"}
		result := ConvertStructToMap(item)
		assert.NotNil(t, result)
		assert.Contains(t, result, "name")
	})

	t.Run("should prefer csv tag over excel tag", func(t *testing.T) {
		type taggedItem struct {
			Name string `csv:"csv_name" excel:"excel_name"`
		}
		item := taggedItem{Name: "Test"}
		result := ConvertStructToMap(&item)
		assert.NotNil(t, result)
		assert.Contains(t, result, "csv_name")
	})

	t.Run("should use excel tag when no csv tag", func(t *testing.T) {
		type taggedItem struct {
			Name string `excel:"excel_name"`
		}
		item := taggedItem{Name: "Test"}
		result := ConvertStructToMap(&item)
		assert.NotNil(t, result)
		assert.Contains(t, result, "excel_name")
	})
}

// ---- GetUpdateTemplate / ExtractUpdateValues (struct_cache.go) ----

func containsCol(cols []string, colName string) bool {
	for _, c := range cols {
		if c == colName+"=?" || c == colName+"=null" || c == colName {
			return true
		}
	}
	return false
}

func TestGetUpdateTemplate(t *testing.T) {
	t.Run("should return template with non-id, non-created_at columns", func(t *testing.T) {
		tmpl := GetUpdateTemplate(reflect.TypeOf(testUser{}))
		assert.NotNil(t, tmpl)
		assert.NotEmpty(t, tmpl.Cols)
		assert.NotEmpty(t, tmpl.FieldPaths)

		// Should not contain created_at or id
		for _, col := range tmpl.Cols {
			assert.NotContains(t, col, "created_at")
		}
	})

	t.Run("should add updated_at automatically when struct lacks it", func(t *testing.T) {
		tmpl := GetUpdateTemplate(reflect.TypeOf(simpleItem{}))
		assert.True(t, tmpl.HaveUpdatedAt || containsCol(tmpl.Cols, "updated_at"))
	})

	t.Run("should not add updated_at when struct has it", func(t *testing.T) {
		tmpl := GetUpdateTemplate(reflect.TypeOf(testUser{}))
		assert.True(t, tmpl.HaveUpdatedAt)
	})
}

func TestExtractUpdateValues(t *testing.T) {
	t.Run("should extract values from struct instance", func(t *testing.T) {
		user := testUser{
			Id:        1,
			Name:      "John",
			Email:     "john@example.com",
			Age:       30,
			CreatedAt: "2024-01-01",
			UpdatedAt: null.StringFrom("2024-01-02"),
		}

		val := reflect.ValueOf(user)
		tmpl := GetUpdateTemplate(val.Type())
		info := ExtractUpdateValues(tmpl, val)

		assert.NotNil(t, info)
		assert.NotEmpty(t, info.Cols)
		assert.NotEmpty(t, info.Values)
	})
}

func TestExtractFixedUpdateValues(t *testing.T) {
	t.Run("should extract all values including nulls", func(t *testing.T) {
		item := nullableItem{
			Id:          1,
			Name:        "Test",
			Description: null.String{}, // Valid=false → nil
		}

		val := reflect.ValueOf(item)
		tmpl := GetUpdateTemplate(val.Type())
		info := ExtractFixedUpdateValues(tmpl, val)

		assert.NotNil(t, info)
		assert.NotEmpty(t, info.Values)
	})

	t.Run("should include nil for null.String with Valid=false", func(t *testing.T) {
		item := nullableItem{
			Id:          1,
			Name:        "Test",
			Description: null.String{}, // Valid=false
		}

		val := reflect.ValueOf(item)
		tmpl := GetUpdateTemplate(val.Type())
		info := ExtractFixedUpdateValues(tmpl, val)

		// Description with Valid=false should produce nil
		hasNil := false
		for _, v := range info.Values {
			if v == nil {
				hasNil = true
			}
		}
		assert.True(t, hasNil, "expected nil value for null.String with Valid=false")
	})

	t.Run("should include empty string for empty string fields", func(t *testing.T) {
		item := nullableItem{
			Id:          1,
			Name:        "",  // empty string
			Description: null.StringFrom("desc"),
		}

		val := reflect.ValueOf(item)
		tmpl := GetUpdateTemplate(val.Type())
		info := ExtractFixedUpdateValues(tmpl, val)

		// Empty string should produce "" (not nil)
		hasEmptyStr := false
		for _, v := range info.Values {
			if v == "" {
				hasEmptyStr = true
			}
		}
		assert.True(t, hasEmptyStr, "expected empty string value for empty Name field")
	})

	t.Run("should append anotherValues", func(t *testing.T) {
		item := simpleItem{
			Id:    1,
			Title: "Test",
			Price: 100,
		}

		val := reflect.ValueOf(item)
		tmpl := GetUpdateTemplate(val.Type())
		info := ExtractFixedUpdateValues(tmpl, val, 42)

		assert.Contains(t, info.Values, 42)
	})
}
