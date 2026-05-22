package db

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

// ============================================================
// GORM — full-featured ORM, most popular in Go ecosystem
// ============================================================

func BenchmarkGorm_Insert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateGormUser(i)
		if err := dbGorm.Create(&u).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	u := GormBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbGorm.Create(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result GormBenchUser
		if err := dbGorm.First(&result, u.ID).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_SelectList(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateGormUser(i)
		dbGorm.Create(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []GormBenchUser
		if err := dbGorm.Where("deleted_at IS NULL").
			Limit(20).Offset(0).
			Find(&users).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_SelectListWithPagination(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateGormUser(i)
		dbGorm.Create(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []GormBenchUser
		page := 1
		limit := 20
		offset := (page - 1) * limit
		if err := dbGorm.Where("deleted_at IS NULL").
			Limit(limit).Offset(offset).
			Find(&users).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_Update(b *testing.B) {
	truncateTable("bench_users")
	u := GormBenchUser{Name: "ToUpdate", Email: "update@bench.dev", Age: 30, Status: "active"}
	dbGorm.Create(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := dbGorm.Model(&u).Updates(map[string]interface{}{
			"name":       fmt.Sprintf("Updated_%d", i),
			"age":        30 + i%10,
			"updated_at": gorm.Expr("NOW()"),
		}).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_Delete(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateGormUser(i)
		dbGorm.Create(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := uint((i % 100) + 1)
		// GORM soft delete
		if err := dbGorm.Delete(&GormBenchUser{}, id).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		users := make([]GormBenchUser, batchSize)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			users[j] = generateGormUser(idx)
		}
		if err := dbGorm.CreateInBatches(users, batchSize).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGorm_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			u := generateGormUser(i)
			if err := dbGorm.Create(&u).Error; err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkGorm_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	u := GormBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbGorm.Create(&u)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result GormBenchUser
			if err := dbGorm.First(&result, u.ID).Error; err != nil {
				b.Fatal(err)
			}
		}
	})
}
