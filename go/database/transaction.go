package database

import (
	"database/sql"
)

type Transactions interface {
	Begin() (*sql.Tx, error)
	Finish(tx *sql.Tx, errTransaction error)
}

type Transaction struct {
	db Master
}

func NewTransaction(db Master) *Transaction {
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
