package db

import (
	"fmt"
	"testing"
)

// ============================================================
// database/sql RAW — absolute baseline (no ORM overhead at all)
// ============================================================

func BenchmarkStdlib_Insert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateUser(i)
		_, err := dbSQL.Exec(
			"INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
			u.Name, u.Email, u.Age, u.Status,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	res, err := dbSQL.Exec("INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
		"SeedUser", "seed@bench.dev", 25, "active")
	if err != nil {
		b.Fatal(err)
	}
	seedID, _ := res.LastInsertId()
	// PostgreSQL BIGSERIAL may not return LastInsertId; fallback to query
	if seedID == 0 {
		_ = dbSQL.QueryRow("SELECT id FROM bench_users WHERE name = "+placeholder(1), "SeedUser").Scan(&seedID)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var u BenchUser
		err := dbSQL.QueryRow(
			"SELECT id, name, email, age, status, created_at, updated_at, deleted_at FROM bench_users WHERE id = "+placeholder(1),
			seedID,
		).Scan(&u.Id, &u.Name, &u.Email, &u.Age, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_SelectList(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		dbSQL.Exec("INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
			u.Name, u.Email, u.Age, u.Status)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rows, err := dbSQL.Query(
			"SELECT id, name, email, age, status, created_at FROM bench_users WHERE deleted_at IS NULL LIMIT 20 OFFSET "+placeholder(1),
			0,
		)
		if err != nil {
			b.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var u BenchUser
			if err := rows.Scan(&u.Id, &u.Name, &u.Email, &u.Age, &u.Status, &u.CreatedAt); err != nil {
				rows.Close()
				b.Fatal(err)
			}
			count++
		}
		rows.Close()
		_ = count
	}
}

func BenchmarkStdlib_Update(b *testing.B) {
	truncateTable("bench_users")
	dbSQL.Exec("INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
		"ToUpdate", "update@bench.dev", 30, "active")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := dbSQL.Exec(
			"UPDATE bench_users SET name="+placeholder(1)+", age="+placeholder(2)+", updated_at=NOW() WHERE id="+placeholder(3),
			fmt.Sprintf("Updated_%d", i), 30+i%10, 1,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_Delete(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		dbSQL.Exec("INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
			u.Name, u.Email, u.Age, u.Status)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := (i % 100) + 1
		_, err := dbSQL.Exec("UPDATE bench_users SET deleted_at=NOW() WHERE id="+placeholder(1), id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		query := "INSERT INTO bench_users (name, email, age, status) VALUES " + bulkPlaceholders(4, batchSize, 1)
		args := make([]interface{}, 0, batchSize*4)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			u := generateUser(idx)
			args = append(args, u.Name, u.Email, u.Age, u.Status)
		}
		_, err := dbSQL.Exec(query, args...)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			u := generateUser(i)
			_, err := dbSQL.Exec(
				"INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
				u.Name, u.Email, u.Age, u.Status,
			)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkStdlib_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	dbSQL.Exec("INSERT INTO bench_users (name, email, age, status) VALUES ("+placeholders(4, 1)+")",
		"SeedUser", "seed@bench.dev", 25, "active")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var u BenchUser
			err := dbSQL.QueryRow(
				"SELECT id, name, email, age, status, created_at FROM bench_users WHERE id = "+placeholder(1),
				1,
			).Scan(&u.Id, &u.Name, &u.Email, &u.Age, &u.Status, &u.CreatedAt)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
