package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlattenJSONB_NoJSONB(t *testing.T) {
	cols := []string{"name", "email"}
	rows := [][]string{{"John", "john@test.com"}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, cols, h)
	assert.Equal(t, rows, r)
}

func TestFlattenJSONB_SingleJSONB(t *testing.T) {
	cols := []string{"name", "phone_number"}
	rows := [][]string{{"John", `{"no_hp":"08123","telp_rumah":"021-xxx"}`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "phone_number_no_hp", "phone_number_telp_rumah"}, h)
	assert.Equal(t, "John", r[0][0])
	assert.Equal(t, "08123", r[0][1])
	assert.Equal(t, "021-xxx", r[0][2])
}

func TestFlattenJSONB_NestedObject(t *testing.T) {
	cols := []string{"name", "document"}
	rows := [][]string{{"John", `{"ktp":{"doc_url":"http://ktp.jpg","no":"123"},"npwp":{"doc_url":"http://npwp.pdf"}}`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "document_ktp_doc_url", "document_ktp_no", "document_npwp_doc_url"}, h)
	assert.Equal(t, "http://ktp.jpg", r[0][1])
	assert.Equal(t, "123", r[0][2])
	assert.Equal(t, "http://npwp.pdf", r[0][3])
}

func TestFlattenJSONB_EmptyJSONB(t *testing.T) {
	cols := []string{"name", "phone_number"}
	rows := [][]string{{"John", ""}, {"Jane", `{"no_hp":"08124"}`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "phone_number_no_hp"}, h)
	assert.Equal(t, "", r[0][1])
	assert.Equal(t, "08124", r[1][1])
}

func TestFlattenJSONB_AllRows(t *testing.T) {
	cols := []string{"name", "data"}
	rows := [][]string{
		{"A", `{"x":"1","y":"2"}`},
		{"B", `{"x":"3","z":"4"}`},
	}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "data_x", "data_y", "data_z"}, h)
	assert.Equal(t, "1", r[0][1])
	assert.Equal(t, "2", r[0][2])
	assert.Equal(t, "", r[0][3])
	assert.Equal(t, "3", r[1][1])
	assert.Equal(t, "", r[1][2])
	assert.Equal(t, "4", r[1][3])
}

func TestFlattenJSONB_NonJSONBraceValue(t *testing.T) {
	cols := []string{"employee_id", "name"}
	rows := [][]string{{"{EMP001 true}", "Budi"}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, cols, h)
	assert.Equal(t, rows, r)
}

func TestFlattenJSONB_ArrayValue(t *testing.T) {
	cols := []string{"name", "documents"}
	rows := [][]string{{"Budi", `[{"type":"ktp","ref_no":"A1"},{"type":"npwp"}]`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "documents_0_ref_no", "documents_0_type", "documents_1_type"}, h)
	assert.Equal(t, "A1", r[0][1])
	assert.Equal(t, "ktp", r[0][2])
	assert.Equal(t, "npwp", r[0][3])
}

func TestFlattenJSONB_ArrayOfObjects(t *testing.T) {
	cols := []string{"data"}
	rows := [][]string{{`[{"a":"1"},{"b":"2"}]`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"data_0_a", "data_1_b"}, h)
	assert.Equal(t, "1", r[0][0])
	assert.Equal(t, "2", r[0][1])
}

func TestFlattenJSONB_JSONNullBecomesEmpty(t *testing.T) {
	cols := []string{"document"}
	rows := [][]string{{`{"ktp":null,"npwp":{"no":"123"}}`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"document_ktp", "document_npwp_no"}, h)
	assert.Equal(t, "", r[0][0])
	assert.Equal(t, "123", r[0][1])
}

func TestFlattenJSONB_AllEmptyObjectsKeptRaw(t *testing.T) {
	cols := []string{"name", "emergency_contact"}
	rows := [][]string{{"A", "{}"}, {"B", "{}"}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, cols, h)
	assert.Equal(t, "{}", r[0][1])
	assert.Equal(t, "{}", r[1][1])
}

func TestFlattenJSONB_MixedInvalidRowInJSONBColumn(t *testing.T) {
	cols := []string{"document"}
	rows := [][]string{{`{"ktp":"1"}`}, {"{broken json}"}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"document_ktp"}, h)
	assert.Equal(t, "1", r[0][0])
	assert.Equal(t, "", r[1][0])
}

func TestFlattenJSONB_EmptyInputs(t *testing.T) {
	h, r := FlattenJSONB(nil, nil)
	assert.Nil(t, h)
	assert.Nil(t, r)
	h, r = FlattenJSONB([]string{"a"}, nil)
	assert.Equal(t, []string{"a"}, h)
	assert.Empty(t, r)
}

func TestFlattenJSONB_ScalarArrayElements(t *testing.T) {
	cols := []string{"data"}
	rows := [][]string{{`["a",1,true,null,[2]]`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"data_0", "data_1", "data_2", "data_3", "data_4_0"}, h)
	assert.Equal(t, []string{"a", "1", "true", "", "2"}, r[0])
}

func TestFlattenJSONB_ScalarObjectValues(t *testing.T) {
	cols := []string{"data"}
	rows := [][]string{{`{"num":1.5,"flag":true,"arr":[1,2]}`}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"data_arr_0", "data_arr_1", "data_flag", "data_num"}, h)
	assert.Equal(t, []string{"1", "2", "true", "1.5"}, r[0])
}

func TestFlattenJSONB_RaggedRows(t *testing.T) {
	cols := []string{"name", "data", "extra"}
	rows := [][]string{{"A", `{"x":"1"}`}, {"B"}}
	h, r := FlattenJSONB(cols, rows)
	assert.Equal(t, []string{"name", "data_x", "extra"}, h)
	assert.Equal(t, "A", r[0][0])
	assert.Equal(t, "1", r[0][1])
	assert.Equal(t, "B", r[1][0])
}
