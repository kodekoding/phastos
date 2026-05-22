package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/volatiletech/null"
)

// BenchUser is the model used across all ORM benchmarks.
// Each ORM will use this struct (or its own mapping) against bench_users table.

type BenchUser struct {
	Id        int64        `json:"id" db:"id"`
	Name      string       `json:"name" db:"name"`
	Email     string       `json:"email" db:"email"`
	Age       int          `json:"age" db:"age"`
	Status    string       `json:"status" db:"status"`
	CreatedAt string       `json:"created_at" db:"created_at"`
	UpdatedAt null.String  `json:"updated_at" db:"updated_at"`
	DeletedAt null.String  `json:"deleted_at" db:"deleted_at"`
}

// GormBenchUser is GORM's version with its own tags + soft-delete support
type GormBenchUser struct {
	ID        uint         `gorm:"primaryKey;autoIncrement"`
	Name      string       `gorm:"size:255;not null"`
	Email     string       `gorm:"size:255;not null"`
	Age       int          `gorm:"default:0"`
	Status    string       `gorm:"size:50;default:active"`
	CreatedAt time.Time
	UpdatedAt *time.Time
	DeletedAt sql.NullTime `gorm:"index"`
}

func (GormBenchUser) TableName() string {
	return "bench_users"
}

// GormBenchProduct is GORM's product model
type GormBenchProduct struct {
	ID        uint         `gorm:"primaryKey;autoIncrement"`
	Name      string       `gorm:"size:255;not null"`
	Price     float64      `gorm:"type:decimal(10,2);default:0"`
	Category  string       `gorm:"size:100;default:''"`
	Stock     int          `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt *time.Time
	DeletedAt sql.NullTime `gorm:"index"`
}

func (GormBenchProduct) TableName() string {
	return "bench_products"
}

// GormBenchOrder is GORM's order model
type GormBenchOrder struct {
	ID        uint         `gorm:"primaryKey;autoIncrement"`
	UserID    uint         `gorm:"not null;index"`
	Total     float64      `gorm:"type:decimal(12,2);default:0"`
	Status    string       `gorm:"size:50;default:pending"`
	CreatedAt time.Time
	UpdatedAt *time.Time
	DeletedAt sql.NullTime `gorm:"index"`
}

func (GormBenchOrder) TableName() string {
	return "bench_orders"
}

// BenchProduct for raw sqlx queries
type BenchProduct struct {
	Id        int64       `json:"id" db:"id"`
	Name      string      `json:"name" db:"name"`
	Price     float64     `json:"price" db:"price"`
	Category  string      `json:"category" db:"category"`
	Stock     int         `json:"stock" db:"stock"`
	CreatedAt string      `json:"created_at" db:"created_at"`
	UpdatedAt null.String `json:"updated_at" db:"updated_at"`
	DeletedAt null.String `json:"deleted_at" db:"deleted_at"`
}

// BenchOrder for raw sqlx queries
type BenchOrder struct {
	Id        int64       `json:"id" db:"id"`
	UserId    int64       `json:"user_id" db:"user_id"`
	Total     float64     `json:"total" db:"total"`
	Status    string      `json:"status" db:"status"`
	CreatedAt string      `json:"created_at" db:"created_at"`
	UpdatedAt null.String `json:"updated_at" db:"updated_at"`
	DeletedAt null.String `json:"deleted_at" db:"deleted_at"`
}

// generateUser creates a test user with a unique suffix
func generateUser(i int) BenchUser {
	return BenchUser{
		Name:   fmt.Sprintf("User_%d", i),
		Email:  fmt.Sprintf("user%d@bench.dev", i),
		Age:    20 + (i % 50),
		Status: "active",
	}
}

// generateGormUser creates a GORM test user
func generateGormUser(i int) GormBenchUser {
	return GormBenchUser{
		Name:   fmt.Sprintf("User_%d", i),
		Email:  fmt.Sprintf("user%d@bench.dev", i),
		Age:    20 + (i % 50),
		Status: "active",
	}
}

// ============================================================
// XORM models
// ============================================================

type XormBenchUser struct {
	Id        int64     `xorm:"'id' pk autoincr"`
	Name      string    `xorm:"'name' varchar(255) notnull"`
	Email     string    `xorm:"'email' varchar(255) notnull"`
	Age       int       `xorm:"'age' int default 0"`
	Status    string    `xorm:"'status' varchar(50) default 'active'"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt *time.Time `xorm:"'updated_at' datetime updated"`
	DeletedAt *time.Time `xorm:"'deleted_at' datetime deleted"`
}

func (XormBenchUser) TableName() string {
	return "bench_users"
}

// generateXormUser creates an XORM test user
func generateXormUser(i int) XormBenchUser {
	return XormBenchUser{
		Name:   fmt.Sprintf("User_%d", i),
		Email:  fmt.Sprintf("user%d@bench.dev", i),
		Age:    20 + (i % 50),
		Status: "active",
	}
}

// ============================================================
// Beego ORM models
// ============================================================

type BeegoBenchUser struct {
	Id        int64     `orm:"pk;auto"`
	Name      string    `orm:"size(255)"`
	Email     string    `orm:"size(255)"`
	Age       int       `orm:"default(0)"`
	Status    string    `orm:"size(50);default(active)"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)"`
	UpdatedAt time.Time `orm:"auto_now;type(datetime);null"`
	DeletedAt time.Time `orm:"type(datetime);null"`
}

func (u *BeegoBenchUser) TableName() string {
	return "bench_users"
}

// generateBeegoUser creates a Beego test user
func generateBeegoUser(i int) BeegoBenchUser {
	return BeegoBenchUser{
		Name:   fmt.Sprintf("User_%d", i),
		Email:  fmt.Sprintf("user%d@bench.dev", i),
		Age:    20 + (i % 50),
		Status: "active",
	}
}

// ============================================================
// Bun models
// ============================================================

type BunBenchUser struct {
	bun.BaseModel `bun:"table:bench_users,alias:u"`

	ID        int64      `bun:"id,pk,autoincrement"`
	Name      string     `bun:"name,notnull"`
	Email     string     `bun:"email,notnull"`
	Age       int        `bun:"age,default:0"`
	Status    string     `bun:"status,default:active"`
	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt *time.Time `bun:"updated_at,nullzero"`
	DeletedAt *time.Time `bun:"deleted_at,nullzero,soft_delete"`
}

// generateBunUser creates a Bun test user
func generateBunUser(i int) BunBenchUser {
	return BunBenchUser{
		Name:   fmt.Sprintf("User_%d", i),
		Email:  fmt.Sprintf("user%d@bench.dev", i),
		Age:    20 + (i % 50),
		Status: "active",
	}
}
