package database

import (
	"sync"
)

// tableRequestPool pools *TableRequest objects used in CUD operations
// (Delete, Upsert, cudProcess) to reduce repeated heap allocations.
var tableRequestPool = sync.Pool{
	New: func() any {
		return new(TableRequest)
	},
}

// GetTableRequest retrieves a *TableRequest from the pool with all fields reset.
func GetTableRequest() *TableRequest {
	//nolint:errcheck // Pool.New always returns *TableRequest
	tr := tableRequestPool.Get().(*TableRequest)
	tr.reset()
	return tr
}

// PutTableRequest returns a *TableRequest to the pool after resetting it.
func PutTableRequest(tr *TableRequest) {
	if tr == nil {
		return
	}
	tr.reset()
	tableRequestPool.Put(tr)
}

// reset clears all mutable fields so the instance is safe to reuse.
func (req *TableRequest) reset() {
	req.Keyword = ""
	req.SearchColsStr = ""
	req.SearchCols = req.SearchCols[:0]
	req.Page = 0
	req.Limit = 0
	req.OrderBy = ""
	req.GroupBy = ""
	req.CreatedStart = ""
	req.CreatedEnd = ""
	req.CustomDateColFilter = ""
	req.InitiateWhere = req.InitiateWhere[:0]
	req.InitiateWhereValues = req.InitiateWhereValues[:0]
	req.IncludeDeleted = false
	req.NotContainsDeletedCol = false
	req.MainTableAlias = ""
	req.IsDeleted = ""
	req.engine = ""
}

// queryOptsPool pools *QueryOpts to reduce per-query heap allocations.
var queryOptsPool = sync.Pool{
	New: func() any {
		return new(QueryOpts)
	},
}

// GetQueryOpts retrieves a *QueryOpts from the pool with all fields reset.
func GetQueryOpts() *QueryOpts {
	//nolint:errcheck // Pool.New always returns *QueryOpts
	o := queryOptsPool.Get().(*QueryOpts)
	o.reset()
	return o
}

// PutQueryOpts returns a *QueryOpts to the pool after resetting it.
func PutQueryOpts(o *QueryOpts) {
	if o == nil {
		return
	}
	o.reset()
	queryOptsPool.Put(o)
}

func (o *QueryOpts) reset() {
	o.BaseQuery = ""
	o.Conditions = nil
	o.ExcludeColumns = ""
	o.Columns = ""
	o.OptionalTableName = ""
	o.SelectRequest = nil
	o.CUDRequest = nil
	o.Result = nil
	o.IsList = false
	o.UpsertInsertId = 0
	o.Trx = nil
	o.LockingType = ""
	o.UseMaster = false
	o.executedQuery = executedQuery{}
}

// selectResponsePool pools *SelectResponse to reduce per-query heap allocations.
var selectResponsePool = sync.Pool{
	New: func() any {
		return new(SelectResponse)
	},
}

// GetSelectResponse retrieves a *SelectResponse from the pool with all fields reset.
func GetSelectResponse() *SelectResponse {
	//nolint:errcheck // Pool.New always returns *SelectResponse
	s := selectResponsePool.Get().(*SelectResponse)
	s.reset()
	return s
}

// PutSelectResponse returns a *SelectResponse to the pool after resetting it.
func PutSelectResponse(s *SelectResponse) {
	if s == nil {
		return
	}
	s.reset()
	selectResponsePool.Put(s)
}

func (s *SelectResponse) reset() {
	s.Data = nil
	if s.ResponseMetaData != nil {
		s.RequestParam = nil
		s.TotalData = 0
		s.TotalFiltered = 0
	}
}

// cudResponsePool pools *CUDResponse to reduce per-write heap allocations.
var cudResponsePool = sync.Pool{
	New: func() any {
		return new(CUDResponse)
	},
}

// GetCUDResponse retrieves a *CUDResponse from the pool with all fields reset.
func GetCUDResponse() *CUDResponse {
	//nolint:errcheck // Pool.New always returns *CUDResponse
	c := cudResponsePool.Get().(*CUDResponse)
	c.reset()
	return c
}

// PutCUDResponse returns a *CUDResponse to the pool after resetting it.
func PutCUDResponse(c *CUDResponse) {
	if c == nil {
		return
	}
	c.reset()
	cudResponsePool.Put(c)
}

func (c *CUDResponse) reset() {
	c.Status = false
	c.RowsAffected = 0
	c.LastInsertID = 0
	c.Message = ""
	c.executedQuery = executedQuery{}
}

// cudDataPool pools *CUDConstructData to reduce per-write heap allocations.
var cudDataPool = sync.Pool{
	New: func() any {
		return new(CUDConstructData)
	},
}

// GetCUDConstructData retrieves a *CUDConstructData from the pool with all fields reset.
func GetCUDConstructData() *CUDConstructData {
	//nolint:errcheck // Pool.New always returns *CUDConstructData
	d := cudDataPool.Get().(*CUDConstructData)
	d.reset()
	return d
}

// PutCUDConstructData returns a *CUDConstructData to the pool after resetting it.
func PutCUDConstructData(d *CUDConstructData) {
	if d == nil {
		return
	}
	d.reset()
	cudDataPool.Put(d)
}

func (d *CUDConstructData) reset() {
	d.Cols = nil
	d.Values = nil
	d.ColsInsert = ""
	d.BulkValues = ""
	d.BulkQuery = ""
	d.Action = ""
	d.TableName = ""
}
