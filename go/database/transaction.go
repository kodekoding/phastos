package database

import (
	"github.com/jmoiron/sqlx"
	plog "github.com/kodekoding/phastos/v2/go/log"
)

type (
	Transactions interface {
		Begin() (*sqlx.Tx, error)
		Finish(tx *sqlx.Tx, errTransaction error)
	}

	Transaction struct {
		db Master
	}
)

func NewTransaction(db Master) *Transaction {
	return &Transaction{db: db}
}

func (t *Transaction) Begin() (*sqlx.Tx, error) {

	return t.db.Beginx()
}

func (t *Transaction) Finish(tx *sqlx.Tx, errTransaction error) {
	var err error
	defer func() {
		if err != nil {
			log := plog.Get()
			log.Err(err).Msg("Got Error when Rollback/Commit Transaction")
		}
	}()
	if errTransaction != nil {
		err = tx.Rollback()
	} else {
		err = tx.Commit()
	}

}
