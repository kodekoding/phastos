package db

import (
	"fmt"
	"testing"

	"github.com/beego/beego/v2/client/orm"
)

// ============================================================
// Beego ORM — ORM from the popular Beego web framework
// ============================================================

func init() {
	orm.RegisterModel(new(BeegoBenchUser))
}

func BenchmarkBeego_Insert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u := generateBeegoUser(i)
		_, err := dbBeego.Insert(&u)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_SelectByID(b *testing.B) {
	truncateTable("bench_users")
	u := BeegoBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbBeego.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := BeegoBenchUser{Id: u.Id}
		err := dbBeego.Read(&result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_SelectList(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateBeegoUser(i)
		dbBeego.Insert(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var users []BeegoBenchUser
		qs := dbBeego.QueryTable("bench_users")
		_, err := qs.Filter("deleted_at__isnull", true).Limit(20).Offset(0).All(&users)
		if err != nil && err != orm.ErrNoRows {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_Update(b *testing.B) {
	truncateTable("bench_users")
	u := BeegoBenchUser{Name: "ToUpdate", Email: "update@bench.dev", Age: 30, Status: "active"}
	dbBeego.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u.Name = fmt.Sprintf("Updated_%d", i)
		u.Age = 30 + i%10
		_, err := dbBeego.Update(&u, "Name", "Age")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_Delete(b *testing.B) {
	truncateTable("bench_users")
	for i := 0; i < 100; i++ {
		u := generateBeegoUser(i)
		dbBeego.Insert(&u)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := int64((i % 100) + 1)
		_, err := dbBeego.Delete(&BeegoBenchUser{Id: id})
		if err != nil && err != orm.ErrNoRows {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_BulkInsert(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	batchSize := 100
	for i := 0; i < b.N; i++ {
		users := make([]*BeegoBenchUser, batchSize)
		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			u := generateBeegoUser(idx)
			users[j] = &u
		}
		_, err := dbBeego.InsertMulti(batchSize, users)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeego_Insert_Parallel(b *testing.B) {
	truncateTable("bench_users")
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			u := generateBeegoUser(i)
			_, err := dbBeego.Insert(&u)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkBeego_SelectByID_Parallel(b *testing.B) {
	truncateTable("bench_users")
	u := BeegoBenchUser{Name: "SeedUser", Email: "seed@bench.dev", Age: 25, Status: "active"}
	dbBeego.Insert(&u)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := BeegoBenchUser{Id: u.Id}
			err := dbBeego.Read(&result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
