# Database Base Action (CRUD)

## How to Use
```go
package main

import (
	"context"
	"log"
	
	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/database/action"
)

func main() {
	db, err := database.Connect(&database.SQLConfig{
		Username:        "root",
		Password:        "...",
		Host:            "localhost",
		Port:            "3306",
		DBName:          "...",
		Engine:          "mysql",
		MaxConnLifetime: 199,
		MaxIdleTime:     100,
		MaxOpenConn:     10,
		MaxIdleConn:     10,
	})

	if err != nil {
		log.Fatalln("error db", err.Error())
	}

	repoOrang := NewOrangRepo(db)
	orangUc := NewUc(repoOrang)

	orangUc.Insert(context.Background(), &OrangEntity{
		Nama:   "test",
		Alamat: "testing",
		Telp:   "2123xx",
	})

	result, err := orangUc.GetList(context.Background(), &OrangRequest{
		TableRequest: database.TableRequest{},
	})

	log.Printf("%#v", result.Data)
}

type (
	Orangs interface {
		Base() *action.Base
	}
	OrangRepo struct {
		base *action.Base
	}
)

func NewOrangRepo(db *database.SQL) *OrangRepo {
	return &OrangRepo{
		base: action.NewBase(db, "orang"),
	}
}

func (o *OrangRepo) Base() *action.Base {
	return o.base
}

type (
	OrangUsecases interface {
		GetList(ctx context.Context, requestData interface{}) (*database.SelectResponse, error)
		Insert(ctx context.Context, requestData interface{}) (*database.CUDResponse, error)
	}
	orangUc struct {
		orangRepo Orangs
	}

	OrangEntity struct {
		database.BaseColumn
		Nama   string `db:"nama"`
		Alamat string `db:"alamat"`
		Telp   string `db:"phone_number"`
	}
	OrangRequest struct {
		OrangEntity
		database.TableRequest
	}
)

func NewUc(repo Orangs) *orangUc {
	return &orangUc{orangRepo: repo}
}

func (o *orangUc) GetList(ctx context.Context, requestData interface{}) (*database.SelectResponse, error) {
	orangRequest := requestData.(*OrangRequest)
	var orangList []*OrangEntity
	if err := o.orangRepo.Base().GetList(ctx, &database.QueryOpts{
		IsList:        true,
		SelectRequest: &orangRequest.TableRequest,
		ResultStruct:  &orangList,
	}); err != nil {
		return nil, err
	}

	result := new(database.SelectResponse)
	result.Data = orangList
	return result, nil
}

func (o *orangUc) Insert(ctx context.Context, requestData interface{}) (*database.CUDResponse, error) {
	result, err := o.orangRepo.Base().Insert(ctx, requestData)
	if err != nil {
		return nil, err
	}
	return result, nil
}



```