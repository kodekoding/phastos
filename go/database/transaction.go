package database

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
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
			log.Warn().Msgf("Got Error when Rollback/Commit Transaction: %s", err.Error())
		}
	}()
	if errTransaction != nil {
		err = tx.Rollback()
	} else {
		err = tx.Commit()
	}

}
