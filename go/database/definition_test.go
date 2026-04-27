package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableRequest_SetWhereCondition(t *testing.T) {
	t.Run("should add condition with value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("name = ?", "John")

		assert.Len(t, req.InitiateWhere, 1)
		assert.Equal(t, "name = ?", req.InitiateWhere[0])
		assert.Len(t, req.InitiateWhereValues, 1)
		assert.Equal(t, "John", req.InitiateWhereValues[0])
	})

	t.Run("should add condition without value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("deleted_at IS NULL")

		assert.Len(t, req.InitiateWhere, 1)
		assert.Equal(t, "deleted_at IS NULL", req.InitiateWhere[0])
		assert.Empty(t, req.InitiateWhereValues)
	})

	t.Run("should accumulate multiple conditions", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("status = ?", "active")
		req.SetWhereCondition("age > ?", 18)
		req.SetWhereCondition("deleted_at IS NULL")

		assert.Len(t, req.InitiateWhere, 3)
		assert.Equal(t, "status = ?", req.InitiateWhere[0])
		assert.Equal(t, "age > ?", req.InitiateWhere[1])
		assert.Equal(t, "deleted_at IS NULL", req.InitiateWhere[2])

		assert.Len(t, req.InitiateWhereValues, 2)
		assert.Equal(t, "active", req.InitiateWhereValues[0])
		assert.Equal(t, 18, req.InitiateWhereValues[1])
	})

	t.Run("should handle multiple values for IN clause", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("id IN (?,?,?)", 1, 2, 3)

		assert.Len(t, req.InitiateWhere, 1)
		assert.Len(t, req.InitiateWhereValues, 3)
		assert.Equal(t, 1, req.InitiateWhereValues[0])
		assert.Equal(t, 2, req.InitiateWhereValues[1])
		assert.Equal(t, 3, req.InitiateWhereValues[2])
	})

	t.Run("should skip nil value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("status = ?", nil)

		assert.Len(t, req.InitiateWhere, 1)
		assert.Empty(t, req.InitiateWhereValues)
	})
}

func TestCUDConstructData_SetValues(t *testing.T) {
	t.Run("should append values", func(t *testing.T) {
		data := &CUDConstructData{}
		data.SetValues("value1")
		data.SetValues(42)
		data.SetValues(true)

		assert.Len(t, data.Values, 3)
		assert.Equal(t, "value1", data.Values[0])
		assert.Equal(t, 42, data.Values[1])
		assert.Equal(t, true, data.Values[2])
	})
}

func TestExecutedQuery_GetGeneratedQuery(t *testing.T) {
	t.Run("should return query and params map", func(t *testing.T) {
		eq := &executedQuery{
			query:  "SELECT * FROM users WHERE id = ?",
			params: []interface{}{1},
		}

		result := eq.GetGeneratedQuery()
		assert.Len(t, result, 1)

		params, exists := result["SELECT * FROM users WHERE id = ?"]
		assert.True(t, exists)
		assert.Equal(t, []interface{}{1}, params)
	})

	t.Run("should handle empty query", func(t *testing.T) {
		eq := &executedQuery{}
		result := eq.GetGeneratedQuery()
		assert.Len(t, result, 1)

		params, exists := result[""]
		assert.True(t, exists)
		assert.Nil(t, params)
	})
}
