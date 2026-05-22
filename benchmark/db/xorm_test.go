package db

import (
	"fmt"
	"testing"
)

// ============================================================
// XORM — popular Go ORM with rich features and cache support
// ============================================================

func BenchmarkXorm_Insert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateXormUser(i)
		_, err := dbXorm.Insert(&u)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	u := XormBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbXorm.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result XormBenchUser
		_, err := dbXorm.ID(u.Id).Get(&result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_SelectList(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateXormUser(i)
		dbXorm.Insert(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []XormBenchUser
		err := dbXorm.Where("deleted_at IS NULL").Limit(20, 0).Find(&users)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_Update(b *testing.B) {
	truncateTable("bench_users")
	u := XormBenchUser{Name: "ToUpdate", Email: "update@bench.dev", Age: 30, Status: "active"}
	dbXorm.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u.Name = fmt.Sprintf("Updated_%d", i)
		u.Age = 30 + i%10
		_, err := dbXorm.ID(u.Id).Cols("name", "age", "updated_at").Update(&u)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_Delete(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateXormUser(i)
		dbXorm.Insert(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := int64((i % 100) + 1)
		_, err := dbXorm.ID(id).Delete(&XormBenchUser{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		users := make([]XormBenchUser, batchSize)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			users[j] = generateXormUser(idx)
		}
		_, err := dbXorm.Insert(&users)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXorm_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			u := generateXormUser(i)
			_, err := dbXorm.Insert(&u)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkXorm_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	u := XormBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbXorm.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result XormBenchUser
			_, err := dbXorm.ID(u.Id).Get(&result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
