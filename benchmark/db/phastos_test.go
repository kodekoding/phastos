package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/database/action"
)

// Create a phastos SQL instance from the existing sqlx connection.
// This mirrors the internal newSQL() function but reuses our test DB.

func newPhastosSQL() *database.SQL {
	sqlObj := &database.SQL{
		Master:   dbSqlx,
		Follower: dbSqlx,
	}
	engine := "mysql"
	if dbType == "postgres" {
		engine = "postgres"
	}
	sqlObj.SetEngine(engine)
	sqlObj.SetSlowQueryThreshold(9999) // disable slow query logging during benchmarks
	return sqlObj
}

// ============================================================
// Phastos ORM — your custom ORM built on top of sqlx
// ============================================================

func BenchmarkPhastos_Insert(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		u := generateUser(i)
		_, err := writer.Insert(ctx, &u)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhastos_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)
	reader := action.NewBaseRead(phastosDB, "bench_users", true)

	ctx := context.Background()

	// Seed one row
	seedUser := BenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	res, err := writer.Insert(ctx, &seedUser)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var u BenchUser
		if err := reader.GetDetailById(ctx, &u, res.LastInsertID); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhastos_SelectList(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)
	reader := action.NewBaseRead(phastosDB, "bench_users", true)

	ctx := context.Background()

	// Seed 100 rows
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		writer.Insert(ctx, &u)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var users []BenchUser
		tableReq := database.GetTableRequest()
		tableReq.Page = 1
		tableReq.Limit = 20

		opts := &database.QueryOpts{
			Result:        &users,
			SelectRequest: tableReq,
		}
		if err := reader.GetList(ctx, opts); err != nil {
			database.PutTableRequest(tableReq)
			b.Fatal(err)
		}
		database.PutTableRequest(tableReq)
	}
}

func BenchmarkPhastos_Update(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	ctx := context.Background()

	seedUser := BenchUser{Name: "ToUpdate", Email: "update@bench.dev", Age: 30, Status: "active"}
	res, err := writer.Insert(ctx, &seedUser)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		updateData := BenchUser{
			Name: fmt.Sprintf("Updated_%d", i),
			Age:  30 + i%10,
		}
		_, err := writer.UpdateById(ctx, &updateData, res.LastInsertID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhastos_Delete(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	ctx := context.Background()

	// Seed 100 rows
	var lastId int64
	for i := 0; i < 100; i++ {
		u := generateUser(i)
		res, _ := writer.Insert(ctx, &u)
		if res != nil {
			lastId = res.LastInsertID
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		id := int64((i % 100) + 1)
		// soft delete by ID
		_, err := writer.DeleteById(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
	}
	_ = lastId
}

func BenchmarkPhastos_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		users := make([]BenchUser, batchSize)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			users[j] = generateUser(idx)
		}
		_, err := writer.BulkInsert(ctx, &users)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhastos_Upsert(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		u := BenchUser{
			Name:   fmt.Sprintf("Upsert_%d", i%10),
			Email:  fmt.Sprintf("upsert%d@bench.dev", i%10),
			Age:    25 + i%10,
			Status: "active",
		}
		condition := map[string]interface{}{
			"email = ?": u.Email,
		}
		_, err := writer.Upsert(ctx, &u, condition)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhastos_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx := context.Background()
			u := generateUser(i)
			_, err := writer.Insert(ctx, &u)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkPhastos_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)
	reader := action.NewBaseRead(phastosDB, "bench_users", true)

	ctx := context.Background()

	seedUser := BenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	res, err := writer.Insert(ctx, &seedUser)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			var u BenchUser
			if err := reader.GetDetailById(ctx, &u, res.LastInsertID); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestVerifyPhastos ensures phastos ORM operations work correctly
func TestVerifyPhastos(t *testing.T) {
	truncateTable("bench_users")
	phastosDB := newPhastosSQL()
	writer := action.NewBaseWrite(phastosDB, "bench_users", true)
	reader := action.NewBaseRead(phastosDB, "bench_users", true)

	ctx := context.Background()

	// Insert
	u := BenchUser{Name: "TestUser", Email: "test@bench.dev", Age: 25, Status: "active"}
	res, err := writer.Insert(ctx, &u)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if !res.Status {
		t.Fatalf("insert status not true")
	}

	// Select by ID
	var selected BenchUser
	if err := reader.GetDetailById(ctx, &selected, res.LastInsertID); err != nil {
		t.Fatalf("select by id failed: %v", err)
	}
	if selected.Name != "TestUser" {
		t.Fatalf("unexpected name: %s", selected.Name)
	}

	// Select List
	var users []BenchUser
	tableReq := database.GetTableRequest()
	defer database.PutTableRequest(tableReq)
	tableReq.Page = 1
	tableReq.Limit = 10

	opts := &database.QueryOpts{
		Result:        &users,
		SelectRequest: tableReq,
	}
	if err := reader.GetList(ctx, opts); err != nil {
		t.Fatalf("select list failed: %v", err)
	}
	if len(users) == 0 {
		t.Fatalf("expected at least 1 user, got 0")
	}

	// Update by ID
	updateData := BenchUser{Name: "Updated", Age: 26}
	_, err = writer.UpdateById(ctx, &updateData, res.LastInsertID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify update
	var updated BenchUser
	if err := reader.GetDetailById(ctx, &updated, res.LastInsertID); err != nil {
		t.Fatalf("select after update failed: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected name 'Updated', got '%s'", updated.Name)
	}

	// Soft delete
	_, err = writer.DeleteById(ctx, res.LastInsertID)
	if err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}
}
