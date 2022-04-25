package common

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/response"
)

type ReadRepo interface {
	GetList(ctx context.Context, opts *database.QueryOpts) error
	GetDetail(ctx context.Context, opts *database.QueryOpts) error
	GetDetailById(ctx context.Context, resultStruct interface{}, id int, optionalTableName ...string) error
	Count(ctx context.Context, tableName string, reqData *database.TableRequest) (totalData, totalFiltered int, err error)
}

type WriteRepo interface {
	Insert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error)
	BulkInsert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error)
	Update(ctx context.Context, data interface{}, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error)
	UpdateById(ctx context.Context, data interface{}, id int, trx ...*sql.Tx) (*database.CUDResponse, error)
	Delete(ctx context.Context, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error)
	DeleteById(ctx context.Context, id int, trx ...*sql.Tx) (*database.CUDResponse, error)
}

type ReadHandler interface {
	GetList(w http.ResponseWriter, r *http.Request) *response.JSON
	GetDetailById(w http.ResponseWriter, r *http.Request) *response.JSON
}

type WriteHandler interface {
	Insert(w http.ResponseWriter, r *http.Request) *response.JSON
	Update(w http.ResponseWriter, r *http.Request) *response.JSON
	Delete(w http.ResponseWriter, r *http.Request) *response.JSON
}

type HandlerCRUD interface {
	ReadHandler
	WriteHandler
}

type RepoCRUD interface {
	ReadRepo
	WriteRepo
}

type UsecaseCRUD interface {
	GetList(ctx context.Context, requestData interface{}) (*database.SelectResponse, error)
	GetDetailById(ctx context.Context, id int) (*database.SelectResponse, error)
	Insert(ctx context.Context, data interface{}) (*database.CUDResponse, error)
	Update(ctx context.Context, data interface{}) (*database.CUDResponse, error)
	Delete(ctx context.Context, id int) (*database.CUDResponse, error)
}
