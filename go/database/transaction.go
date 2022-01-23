package database

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type Transactions interface {
	Begin() (*sql.Tx, error)
	Finish(tx *sql.Tx, errTransaction error)
}

type Transaction struct {
	db *sqlx.DB
}

func NewTransaction(db *sqlx.DB) *Transaction {
	return &Transaction{db: db}
}

func (t *Transaction) Begin() (*sql.Tx, error) {
	return t.db.Begin()
}

func (t *Transaction) Finish(tx *sql.Tx, errTransaction error) {
	if errTransaction != nil {
		_ = tx.Rollback()
	} else {
		_ = tx.Commit()
	}
}
