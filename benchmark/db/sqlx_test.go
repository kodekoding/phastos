package db

import (
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// sqlx — near-baseline (struct scanning, named queries, etc.)
// Uses ? placeholders + Rebind() to auto-convert for PostgreSQL
// ============================================================

func BenchmarkSqlx_Insert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateUser(i)
		_, err := dbSqlx.Exec(
			dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
			u.Name, u.Email, u.Age, u.Status,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	res, err := dbSqlx.Exec(dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
		"SeedUser", "seed@bench.dev", 25, "active")
	if err != nil {
		b.Fatal(err)
	}
	seedID, _ := res.LastInsertId()
	if seedID == 0 {
		_ = dbSqlx.QueryRow(dbSqlx.Rebind("SELECT id FROM bench_users WHERE name = ?"), "SeedUser").Scan(&seedID)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var u BenchUser
		err := sqlx.Get(dbSqlx, &u, dbSqlx.Rebind("SELECT * FROM bench_users WHERE id = ?"), seedID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_SelectList(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		dbSqlx.Exec(dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
			u.Name, u.Email, u.Age, u.Status)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []BenchUser
		err := sqlx.Select(dbSqlx, &users,
			dbSqlx.Rebind("SELECT * FROM bench_users WHERE deleted_at IS NULL LIMIT 20 OFFSET ?"), 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_Update(b *testing.B) {
	truncateTable("bench_users")
	dbSqlx.Exec(dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
		"ToUpdate", "update@bench.dev", 30, "active")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := dbSqlx.Exec(
			dbSqlx.Rebind("UPDATE bench_users SET name=?, age=?, updated_at=NOW() WHERE id=?"),
			fmt.Sprintf("Updated_%d", i), 30+i%10, 1,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_Delete(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		dbSqlx.Exec(dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
			u.Name, u.Email, u.Age, u.Status)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := (i % 100) + 1
		_, err := dbSqlx.Exec(dbSqlx.Rebind("UPDATE bench_users SET deleted_at=NOW() WHERE id=?"), id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		query := "INSERT INTO bench_users (name, email, age, status) VALUES "
		args := make([]interface{}, 0, batchSize*4)
		for j := 0; j < batchSize; j++ {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?, ?, ?)"
			idx := i*batchSize + j
			u := generateUser(idx)
			args = append(args, u.Name, u.Email, u.Age, u.Status)
		}
		_, err := dbSqlx.Exec(dbSqlx.Rebind(query), args...)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSqlx_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			u := generateUser(i)
			_, err := dbSqlx.Exec(
				dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
				u.Name, u.Email, u.Age, u.Status,
			)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkSqlx_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	dbSqlx.Exec(dbSqlx.Rebind("INSERT INTO bench_users (name, email, age, status) VALUES (?, ?, ?, ?)"),
		"SeedUser", "seed@bench.dev", 25, "active")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var u BenchUser
			err := sqlx.Get(dbSqlx, &u, dbSqlx.Rebind("SELECT * FROM bench_users WHERE id = ?"), 1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
