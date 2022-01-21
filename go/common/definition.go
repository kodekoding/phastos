package common

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/response"
)

type ReadRepo interface {
	GetList(ctx context.Context, requestData interface{}) (interface{}, error)
	GetDetailById(ctx context.Context, id int) (interface{}, error)
}

type WriteRepo interface {
	Insert(ctx context.Context, data *database.CUDConstructData, trx ...*sql.Tx) (*database.CUDResponse, error)
	Update(ctx context.Context, data *database.CUDConstructData, trx ...*sql.Tx) (*database.CUDResponse, error)
	Delete(ctx context.Context, id int, trx ...*sql.Tx) (*database.CUDResponse, error)
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
