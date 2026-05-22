package database

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- TableRequest pool tests ---

func TestTableRequestPoolReuse(t *testing.T) {
	tr := GetTableRequest()
	tr.Keyword = "pool-search"
	tr.SearchCols = []string{"name", "email"}
	tr.Page = 5
	tr.Limit = 10
	tr.OrderBy = "created_at DESC"
	tr.GroupBy = "status"
	tr.CreatedStart = "2024-01-01"
	tr.CreatedEnd = "2024-12-31"
	tr.CustomDateColFilter = "updated_at"
	tr.InitiateWhere = []string{"id = ?"}
	tr.InitiateWhereValues = []interface{}{123}
	tr.IncludeDeleted = true
	tr.NotContainsDeletedCol = true
	tr.MainTableAlias = "t"
	tr.IsDeleted = "1"
	tr.engine = "mysql"

	PutTableRequest(tr)

	tr2 := GetTableRequest()
	assert.Equal(t, "", tr2.Keyword)
	assert.Len(t, tr2.SearchCols, 0)
	assert.Equal(t, 0, tr2.Page)
	assert.Equal(t, 0, tr2.Limit)
	assert.Equal(t, "", tr2.OrderBy)
	assert.Equal(t, "", tr2.GroupBy)
	assert.Equal(t, "", tr2.CreatedStart)
	assert.Equal(t, "", tr2.CreatedEnd)
	assert.Equal(t, "", tr2.CustomDateColFilter)
	assert.Len(t, tr2.InitiateWhere, 0)
	assert.Len(t, tr2.InitiateWhereValues, 0)
	assert.False(t, tr2.IncludeDeleted)
	assert.False(t, tr2.NotContainsDeletedCol)
	assert.Equal(t, "", tr2.MainTableAlias)
	assert.Equal(t, "", tr2.IsDeleted)
	assert.Equal(t, "", tr2.engine)

	PutTableRequest(tr2)
}

func TestTableRequestPoolConcurrency(t *testing.T) {
	const workers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tr := GetTableRequest()
				tr.Keyword = "test"
				tr.Page = id
				tr.Limit = j
				tr.InitiateWhere = append(tr.InitiateWhere, "id = ?")
				tr.InitiateWhereValues = append(tr.InitiateWhereValues, id)
				PutTableRequest(tr)
			}
		}(i)
	}

	wg.Wait()
}

func TestPutTableRequestNil(t *testing.T) {
	assert.NotPanics(t, func() {
		PutTableRequest(nil)
	})
}

// --- QueryOpts pool tests ---

func TestQueryOptsPoolReuse(t *testing.T) {
	o := GetQueryOpts()
	o.BaseQuery = "SELECT * FROM foo"
	o.ExcludeColumns = "password"
	o.Columns = "id,name"
	o.OptionalTableName = "bar"
	o.IsList = true
	o.UpsertInsertId = 99
	o.LockingType = "FOR UPDATE"
	o.UseMaster = true
	o.executedQuery = executedQuery{query: "q", params: []interface{}{1}}

	PutQueryOpts(o)

	o2 := GetQueryOpts()
	assert.Equal(t, "", o2.BaseQuery)
	assert.Equal(t, "", o2.ExcludeColumns)
	assert.Equal(t, "", o2.Columns)
	assert.Equal(t, "", o2.OptionalTableName)
	assert.False(t, o2.IsList)
	assert.Equal(t, int64(0), o2.UpsertInsertId)
	assert.Equal(t, "", o2.LockingType)
	assert.False(t, o2.UseMaster)
	assert.Nil(t, o2.SelectRequest)
	assert.Nil(t, o2.CUDRequest)
	assert.Nil(t, o2.Result)
	assert.Nil(t, o2.Trx)
	assert.Equal(t, "", o2.query)
	assert.Len(t, o2.params, 0)
	PutQueryOpts(o2)
}

func TestQueryOptsReset_ClearsConditions(t *testing.T) {
	o := GetQueryOpts()
	o.Conditions = func(ctx context.Context) {}
	o.SelectRequest = &TableRequest{Keyword: "test"}
	o.CUDRequest = &CUDConstructData{Action: ActionInsert}

	o.reset()
	assert.Nil(t, o.Conditions)
	assert.Nil(t, o.SelectRequest)
	assert.Nil(t, o.CUDRequest)
	assert.Nil(t, o.Result)
	PutQueryOpts(o)
}

func TestPutQueryOptsNil(t *testing.T) {
	assert.NotPanics(t, func() {
		PutQueryOpts(nil)
	})
}

func TestQueryOptsPoolConcurrency(t *testing.T) {
	const workers = 30
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				o := GetQueryOpts()
				o.BaseQuery = "SELECT 1"
				o.IsList = id%2 == 0
				o.UseMaster = j%3 == 0
				PutQueryOpts(o)
			}
		}(i)
	}

	wg.Wait()
}

// --- SelectResponse pool tests ---

func TestSelectResponsePoolReuse(t *testing.T) {
	s := GetSelectResponse()
	s.Data = []string{"a", "b"}
	s.ResponseMetaData = &ResponseMetaData{RequestParam: "p", TotalData: 10, TotalFiltered: 5}

	PutSelectResponse(s)

	s2 := GetSelectResponse()
	assert.Nil(t, s2.Data)
	// ResponseMetaData is not nil because it was set and reset preserves the pointer
	if s2.ResponseMetaData != nil {
		assert.Nil(t, s2.RequestParam)
		assert.Equal(t, int64(0), s2.TotalData)
		assert.Equal(t, int64(0), s2.TotalFiltered)
	}
	PutSelectResponse(s2)
}

func TestSelectResponseReset_NoMetaData(t *testing.T) {
	s := GetSelectResponse()
	s.Data = []int{1, 2, 3}
	// No ResponseMetaData set

	s.reset()
	assert.Nil(t, s.Data)
	PutSelectResponse(s)
}

func TestPutSelectResponseNil(t *testing.T) {
	assert.NotPanics(t, func() {
		PutSelectResponse(nil)
	})
}

// --- CUDResponse pool tests ---

func TestCUDResponsePoolReuse(t *testing.T) {
	c := GetCUDResponse()
	c.Status = true
	c.RowsAffected = 42
	c.LastInsertID = 7
	c.Message = "ok"
	c.executedQuery = executedQuery{query: "q", params: []interface{}{1}}

	PutCUDResponse(c)

	c2 := GetCUDResponse()
	assert.False(t, c2.Status)
	assert.Equal(t, int64(0), c2.RowsAffected)
	assert.Equal(t, int64(0), c2.LastInsertID)
	assert.Equal(t, "", c2.Message)
	assert.Equal(t, "", c2.executedQuery.query)
	PutCUDResponse(c2)
}

func TestPutCUDResponseNil(t *testing.T) {
	assert.NotPanics(t, func() {
		PutCUDResponse(nil)
	})
}

func TestCUDResponsePoolConcurrency(t *testing.T) {
	const workers = 30
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				c := GetCUDResponse()
				c.Status = true
				c.RowsAffected = int64(id)
				c.LastInsertID = int64(j)
				PutCUDResponse(c)
			}
		}(i)
	}

	wg.Wait()
}

// --- CUDConstructData pool tests ---

func TestCUDConstructDataPoolReuse(t *testing.T) {
	d := GetCUDConstructData()
	d.Cols = []string{"name", "email"}
	d.Values = []interface{}{"John", "john@example.com"}
	d.ColsInsert = "name,email"
	d.BulkValues = "(?,?),(?,?)"
	d.BulkQuery = "SET name = VALUES(name)"
	d.Action = ActionInsert
	d.TableName = "users"

	PutCUDConstructData(d)

	d2 := GetCUDConstructData()
	assert.Nil(t, d2.Cols)
	assert.Nil(t, d2.Values)
	assert.Equal(t, "", d2.ColsInsert)
	assert.Equal(t, "", d2.BulkValues)
	assert.Equal(t, "", d2.BulkQuery)
	assert.Equal(t, "", d2.Action)
	assert.Equal(t, "", d2.TableName)
	PutCUDConstructData(d2)
}

func TestPutCUDConstructDataNil(t *testing.T) {
	assert.NotPanics(t, func() {
		PutCUDConstructData(nil)
	})
}

func TestCUDConstructDataPoolConcurrency(t *testing.T) {
	const workers = 30
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				d := GetCUDConstructData()
				d.Cols = []string{"col1"}
				d.Values = []interface{}{id}
				d.Action = ActionUpdate
				d.TableName = "test"
				PutCUDConstructData(d)
			}
		}(i)
	}

	wg.Wait()
}

// --- TableRequest reset test (direct) ---

func TestTableRequestReset(t *testing.T) {
	tr := &TableRequest{
		Keyword:               "search",
		SearchColsStr:         "name",
		SearchCols:            []string{"name"},
		Page:                  5,
		Limit:                 10,
		OrderBy:               "id",
		GroupBy:               "status",
		CreatedStart:          "2024-01-01",
		CreatedEnd:            "2024-12-31",
		CustomDateColFilter:   "updated_at",
		InitiateWhere:         []string{"id = ?"},
		InitiateWhereValues:   []interface{}{1},
		IncludeDeleted:        true,
		NotContainsDeletedCol: true,
		MainTableAlias:        "t",
		IsDeleted:             "1",
		engine:                "mysql",
	}

	tr.reset()

	assert.Equal(t, "", tr.Keyword)
	assert.Equal(t, "", tr.SearchColsStr)
	assert.Len(t, tr.SearchCols, 0)
	assert.Equal(t, 0, tr.Page)
	assert.Equal(t, 0, tr.Limit)
	assert.Equal(t, "", tr.OrderBy)
	assert.Equal(t, "", tr.GroupBy)
	assert.Equal(t, "", tr.CreatedStart)
	assert.Equal(t, "", tr.CreatedEnd)
	assert.Equal(t, "", tr.CustomDateColFilter)
	assert.Len(t, tr.InitiateWhere, 0)
	assert.Len(t, tr.InitiateWhereValues, 0)
	assert.False(t, tr.IncludeDeleted)
	assert.False(t, tr.NotContainsDeletedCol)
	assert.Equal(t, "", tr.MainTableAlias)
	assert.Equal(t, "", tr.IsDeleted)
	assert.Equal(t, "", tr.engine)
}

// --- CUDConstructData reset test (direct) ---

func TestCUDConstructDataReset(t *testing.T) {
	d := &CUDConstructData{
		Cols:       []string{"name"},
		Values:     []interface{}{"John"},
		ColsInsert: "name",
		BulkValues: "(?)",
		BulkQuery:  "SET name=VALUES(name)",
		Action:     ActionInsert,
		TableName:  "users",
	}

	d.reset()

	assert.Nil(t, d.Cols)
	assert.Nil(t, d.Values)
	assert.Equal(t, "", d.ColsInsert)
	assert.Equal(t, "", d.BulkValues)
	assert.Equal(t, "", d.BulkQuery)
	assert.Equal(t, "", d.Action)
	assert.Equal(t, "", d.TableName)
}

// --- Benchmark tests ---

func BenchmarkTableRequestPool(b *testing.B) {
	b.Run("pooled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tr := GetTableRequest()
			tr.Keyword = "search"
			tr.Page = i % 100
			tr.Limit = 20
			tr.SetWhereCondition("id = ?", i)
			PutTableRequest(tr)
		}
	})

	b.Run("direct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tr := new(TableRequest)
			tr.Keyword = "search"
			tr.Page = i % 100
			tr.Limit = 20
			tr.SetWhereCondition("id = ?", i)
			_ = tr
		}
	})
}

func BenchmarkCUDConstructDataPool(b *testing.B) {
	b.Run("pooled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := GetCUDConstructData()
			d.Cols = []string{"name"}
			d.Values = []interface{}{"John"}
			d.Action = ActionInsert
			d.TableName = "users"
			PutCUDConstructData(d)
		}
	})

	b.Run("direct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := new(CUDConstructData)
			d.Cols = []string{"name"}
			d.Values = []interface{}{"John"}
			d.Action = ActionInsert
			d.TableName = "users"
			_ = d
		}
	})
}
