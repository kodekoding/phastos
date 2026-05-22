package db

import (
	"context"
	"fmt"
	"testing"
)

// ============================================================
// Bun — modern SQL-first ORM, lightweight and high-performance
// ============================================================

func BenchmarkBun_Insert(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateBunUser(i)
		_, err := dbBun.NewInsert().Model(&u).Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	u := BunBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbBun.NewInsert().Model(&u).Exec(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result BunBenchUser
		err := dbBun.NewSelect().Model(&result).Where("id = ?", u.ID).Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_SelectList(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		u := generateBunUser(i)
		dbBun.NewInsert().Model(&u).Exec(ctx)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []BunBenchUser
		err := dbBun.NewSelect().
			Model(&users).
			Where("deleted_at IS NULL").
			Limit(20).
			Offset(0).
			Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_Update(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	u := BunBenchUser{Name: "ToUpdate", Email: "update@bench.dev", Age: 30, Status: "active"}
	dbBun.NewInsert().Model(&u).Exec(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := dbBun.NewUpdate().
			Model(&BunBenchUser{}).
			Set("name = ?", fmt.Sprintf("Updated_%d", i)).
			Set("age = ?", 30+i%10).
			Set("updated_at = NOW()").
			Where("id = ?", u.ID).
			Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_Delete(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		u := generateBunUser(i)
		dbBun.NewInsert().Model(&u).Exec(ctx)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := int64((i % 100) + 1)
		// soft delete
		_, err := dbBun.NewUpdate().
			Model(&BunBenchUser{}).
			Set("deleted_at = NOW()").
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		users := make([]BunBenchUser, batchSize)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			users[j] = generateBunUser(idx)
		}
		_, err := dbBun.NewInsert().Model(&users).Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBun_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx := context.Background()
			u := generateBunUser(i)
			_, err := dbBun.NewInsert().Model(&u).Exec(ctx)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkBun_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	ctx := context.Background()
	u := BunBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbBun.NewInsert().Model(&u).Exec(ctx)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			var result BunBenchUser
			err := dbBun.NewSelect().Model(&result).Where("id = ?", u.ID).Scan(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
