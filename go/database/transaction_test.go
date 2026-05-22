package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewTransaction tests ---

func TestNewTransaction(t *testing.T) {
	master := &stubDB{}
	txn := NewTransaction(master)
	assert.NotNil(t, txn)
	assert.Equal(t, master, txn.db)
}

func TestNewTransaction_NilDB(t *testing.T) {
	// Creating a transaction with nil db should not panic at creation time
	txn := NewTransaction(nil)
	assert.NotNil(t, txn)
}

// --- Transaction.Begin tests ---

func TestTransaction_Begin_Error(t *testing.T) {
	master := &stubDB{beginxError: sql.ErrConnDone}
	txn := NewTransaction(master)
	tx, err := txn.Begin()
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, tx)
}

func TestTransaction_Begin_DelegatesToMaster(t *testing.T) {
	expectedErr := sql.ErrConnDone
	master := &stubDB{beginxError: expectedErr}
	txn := NewTransaction(master)

	tx, err := txn.Begin()
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, tx)
}

// --- Transaction.Finish tests with a real *sqlx.Tx ---
// Uses the sql_test_fixture driver registered in sql_test.go

func TestTransaction_Begin_Success(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Clean up - rollback the transaction
	txn.Finish(tx, sql.ErrTxDone)
}

func TestTransaction_Finish_Commit(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Finish with no error should commit
	txn.Finish(tx, nil)
}

func TestTransaction_Finish_Rollback(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Finish with error should rollback
	txn.Finish(tx, sql.ErrTxDone)
}

func TestTransaction_Finish_CommitAndRollback_Sequential(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)

	// First transaction: commit
	tx1, err := txn.Begin()
	require.NoError(t, err)
	txn.Finish(tx1, nil)

	// Second transaction: rollback
	tx2, err := txn.Begin()
	require.NoError(t, err)
	txn.Finish(tx2, sql.ErrConnDone)
}

func TestTransaction_Begin_MultipleTimes(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)

	// Each Begin should create a new transaction
	tx1, err := txn.Begin()
	require.NoError(t, err)

	tx2, err := txn.Begin()
	require.NoError(t, err)

	// They should be different instances
	assert.NotEqual(t, tx1, tx2)

	// Clean up
	txn.Finish(tx1, nil)
	txn.Finish(tx2, nil)
}
