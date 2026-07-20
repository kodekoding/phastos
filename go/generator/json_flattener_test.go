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
