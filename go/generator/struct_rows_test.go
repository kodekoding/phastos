package generator

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
	"github.com/xuri/excelize/v2"
)

type TestStruct struct {
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

type TestEmbeddedStruct struct {
	TestStruct
	Department string `db:"department"`
}

type testUnTaggedStruct struct {
	ID    int    `db:"id"`
	Title string `json:"title"`
}

// =============================================================================
// WriteToBuffer Tests
// =============================================================================

func TestWriteToBuffer_WithContent(t *testing.T) {
	g := NewExcel(&ExcelOptions{Source: "new"})
	g.SetSheetName("Test").SetHeader([]string{"A", "B"})
	g.AppendDataRow([]string{"1", "2"})
	require.NoError(t, g.Error())

	buf, err := g.WriteToBuffer()
	require.NoError(t, err)
	require.NotNil(t, buf)

	data := buf.Bytes()
	assert.Greater(t, len(data), 0)
	assert.Equal(t, []byte("PK"), data[:2])
}

func TestWriteToBuffer_EmptyContent(t *testing.T) {
	g := NewExcel(&ExcelOptions{Source: "new"})
	g.SetSheetName("Test")
	require.NoError(t, g.Error())

	buf, err := g.WriteToBuffer()
	require.NoError(t, err)
	require.NotNil(t, buf)

	data := buf.Bytes()
	assert.Greater(t, len(data), 0)
	assert.Equal(t, []byte("PK"), data[:2])
}

func TestWriteToBuffer_PreExistingError(t *testing.T) {
	g := NewExcel(&ExcelOptions{Source: "new"})
	g.SetSheetName("Test").SetHeader([]string{"A"})
	g.AppendDataRow([]string{"1", "2"})

	buf, err := g.WriteToBuffer()
	assert.Error(t, err)
	assert.Nil(t, buf)
	assert.Contains(t, err.Error(), "Total Column")
}

func TestWriteToBuffer_ValidXLSX(t *testing.T) {
	g := NewExcel(&ExcelOptions{Source: "new"})
	g.SetSheetName("Sheet1").SetHeader([]string{"Name", "Value"})
	g.AppendDataRow([]string{"Alice", "100"})
	g.AppendDataRow([]string{"Bob", "200"})
	require.NoError(t, g.Error())

	buf, err := g.WriteToBuffer()
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"Name", "Value"}, rows[0])
	assert.Equal(t, []string{"Alice", "100"}, rows[1])
	assert.Equal(t, []string{"Bob", "200"}, rows[2])
}

func TestWriteToBuffer_MultipleContentWrites(t *testing.T) {
	g := NewExcel(&ExcelOptions{Source: "new"})
	g.SetSheetName("Data")
	g.SetHeader([]string{"col1", "col2", "col3"})

	for i := 0; i < 50; i++ {
		g.AppendDataRow([]string{"a", "b", "c"})
	}
	require.NoError(t, g.Error())

	buf, err := g.WriteToBuffer()
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer f.Close()

	rows, err := f.GetRows("Data")
	require.NoError(t, err)
	require.Len(t, rows, 51)
}

// =============================================================================
// StructRows Tests
// =============================================================================

func TestStructRows_Basic(t *testing.T) {
	data := []*TestStruct{
		{Name: "Alice", Email: "alice@test.com", Age: 30},
		{Name: "Bob", Email: "bob@test.com", Age: 25},
	}
	rows, err := StructRows([]string{"name", "email", "age"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, []string{"Alice", "alice@test.com", "30"}, rows[0])
	assert.Equal(t, []string{"Bob", "bob@test.com", "25"}, rows[1])
}

func TestStructRows_EmptySlice(t *testing.T) {
	var data []*TestStruct
	rows, err := StructRows([]string{"name"}, data)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStructRows_NilStruct(t *testing.T) {
	data := []*TestStruct{nil, {Name: "Alice"}}
	rows, err := StructRows([]string{"name"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, []string{""}, rows[0])
	assert.Equal(t, []string{"Alice"}, rows[1])
}

func TestStructRows_NonPointerSlice(t *testing.T) {
	data := []TestStruct{
		{Name: "Alice", Email: "alice@test.com"},
	}
	rows, err := StructRows([]string{"name", "email"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, []string{"Alice", "alice@test.com"}, rows[0])
}

func TestStructRows_NotSlice(t *testing.T) {
	_, err := StructRows([]string{"name"}, "not a slice")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a slice")
}

func TestStructRows_EmptyColumns(t *testing.T) {
	_, err := StructRows([]string{}, []*TestStruct{{Name: "A"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestStructRows_EmbeddedStruct(t *testing.T) {
	data := []*TestEmbeddedStruct{
		{TestStruct: TestStruct{Name: "Alice", Email: "a@test.com"}, Department: "Engineering"},
	}
	rows, err := StructRows([]string{"name", "email", "department"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, []string{"Alice", "a@test.com", "Engineering"}, rows[0])
}

func TestStructRows_LargeDataSet(t *testing.T) {
	n := 1000
	data := make([]*TestStruct, n)
	for i := 0; i < n; i++ {
		data[i] = &TestStruct{Name: "User", Email: "user@test.com", Age: i}
	}
	rows, err := StructRows([]string{"name", "email", "age"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, n)
	assert.Equal(t, "0", rows[0][2])
}

func TestStructRows_NonStructElement(t *testing.T) {
	data := []interface{}{"not a struct"}
	rows, err := StructRows([]string{"col"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, []string{""}, rows[0])
}

func TestStructRows_OnlyDbTaggedFields(t *testing.T) {
	data := []testUnTaggedStruct{{ID: 42, Title: "Hello"}}
	rows, err := StructRows([]string{"id"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "42", rows[0][0])
}

func TestStructRows_ColumnNotInStruct(t *testing.T) {
	data := []*TestStruct{{Name: "Alice"}}
	rows, err := StructRows([]string{"name", "nonexistent"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Alice", rows[0][0])
	assert.Equal(t, "", rows[0][1])
}

// =============================================================================
// structFieldToString Tests (via StructRows)
// =============================================================================

func TestStructFieldToString_NullPointer(t *testing.T) {
	var ptr *TestStruct
	rows, err := StructRows([]string{"name"}, []*TestStruct{ptr})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, []string{""}, rows[0])
}

// =============================================================================
// collectDBTaggedFields Tests (via StructRows)
// =============================================================================

func TestCollectDBTaggedFields_NonStructAnonymous(t *testing.T) {
	type Inner struct {
		Value string `db:"value"`
	}
	type Outer struct {
		Inner
		Label string `db:"label"`
	}
	data := []Outer{{Inner: Inner{Value: "v"}, Label: "l"}}
	rows, err := StructRows([]string{"value", "label"}, data)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "v", rows[0][0])
	assert.Equal(t, "l", rows[0][1])
}

// =============================================================================
// structFieldToString indirect coverage Tests
// =============================================================================

type intField struct {
	Count int `db:"count"`
}

type floatField struct {
	Amount float64 `db:"amount"`
}

type boolField struct {
	Active bool `db:"active"`
}

type ptrField struct {
	Name *string `db:"name"`
}

func TestStructFieldToString_PlainInt(t *testing.T) {
	rows, err := StructRows([]string{"count"}, []intField{{Count: 42}})
	require.NoError(t, err)
	assert.Equal(t, "42", rows[0][0])
}

func TestStructFieldToString_PlainFloat(t *testing.T) {
	rows, err := StructRows([]string{"amount"}, []floatField{{Amount: 3.14}})
	require.NoError(t, err)
	assert.Equal(t, "3.14", rows[0][0])
}

func TestStructFieldToString_PlainBool(t *testing.T) {
	rows, err := StructRows([]string{"active"}, []boolField{{Active: true}})
	require.NoError(t, err)
	assert.Equal(t, "true", rows[0][0])
}

func TestStructFieldToString_NonNilPointer(t *testing.T) {
	s := "hello"
	rows, err := StructRows([]string{"name"}, []ptrField{{Name: &s}})
	require.NoError(t, err)
	assert.Equal(t, "hello", rows[0][0])
}

func TestStructFieldToString_NilPointerField(t *testing.T) {
	rows, err := StructRows([]string{"name"}, []ptrField{{Name: nil}})
	require.NoError(t, err)
	assert.Equal(t, "", rows[0][0])
}

type NullField struct {
	Data null.String `db:"data"`
}

func TestStructFieldToString_ValuerNonNull(t *testing.T) {
	rows, err := StructRows([]string{"data"}, []NullField{{Data: null.StringFrom("hello")}})
	require.NoError(t, err)
	assert.Equal(t, "hello", rows[0][0])
}

func TestStructFieldToString_ValuerNull(t *testing.T) {
	rows, err := StructRows([]string{"data"}, []NullField{{Data: null.String{}}})
	require.NoError(t, err)
	assert.Equal(t, "", rows[0][0])
}
