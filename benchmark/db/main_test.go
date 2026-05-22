package db

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"xorm.io/xorm"
)

var (
	dbSQL   *sql.DB
	dbSqlx  *sqlx.DB
	dbGorm  *gorm.DB
	dbXorm  *xorm.Engine
	dbBeego orm.Ormer
	dbBun   *bun.DB

	dsn    string
	dbType string // "mysql" or "postgres"
	driver string // "mysql" or "postgres"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	dbType = os.Getenv("BENCHMARK_DB_TYPE")
	if dbType == "" {
		dbType = "mysql"
	}

	dsn = os.Getenv("BENCHMARK_DB_DSN")
	if dsn == "" {
		switch dbType {
		case "postgres":
			dsn = "postgres://postgres:postgres@127.0.0.1:32771/phastos_benchmark?sslmode=disable"
		default:
			dsn = "root:toor@tcp(127.0.0.1:3306)/phastos_benchmark?parseTime=true&multiStatements=true"
		}
	}

	switch dbType {
	case "postgres":
		driver = "postgres"
	default:
		driver = "mysql"
	}

	fmt.Printf("Benchmark DB: %s | DSN: %s\n", dbType, dsn)

	var err error

	// Connect raw database/sql
	dbSQL, err = sql.Open(driver, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to open raw sql: %v\n", err)
		os.Exit(1)
	}
	dbSQL.SetMaxOpenConns(4)
	dbSQL.SetMaxIdleConns(4)
	dbSQL.SetConnMaxLifetime(5 * time.Minute)

	if err := dbSQL.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to ping %s: %v\n", dbType, err)
		os.Exit(1)
	}

	// Connect sqlx
	dbSqlx, err = sqlx.Connect(driver, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to open sqlx: %v\n", err)
		os.Exit(1)
	}
	dbSqlx.SetMaxOpenConns(4)
	dbSqlx.SetMaxIdleConns(4)
	dbSqlx.SetConnMaxLifetime(5 * time.Minute)

	// Connect GORM
	switch dbType {
	case "postgres":
		dbGorm, err = gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	default:
		dbGorm, err = gorm.Open(gormmysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to open gorm: %v\n", err)
		os.Exit(1)
	}
	sqlDB, _ := dbGorm.DB()
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Connect XORM
	dbXorm, err = xorm.NewEngine(driver, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to open xorm: %v\n", err)
		os.Exit(1)
	}
	dbXorm.SetMaxOpenConns(4)
	dbXorm.SetMaxIdleConns(4)
	dbXorm.SetConnMaxLifetime(5 * time.Minute)
	dbXorm.ShowSQL(false)

	// Connect Beego ORM
	err = orm.RegisterDataBase("default", driver, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to open beego orm: %v\n", err)
		os.Exit(1)
	}
	dbBeego = orm.NewOrm()

	// Connect Bun
	switch dbType {
	case "postgres":
		dbBun = bun.NewDB(dbSQL, pgdialect.New())
	default:
		dbBun = bun.NewDB(dbSQL, mysqldialect.New())
	}

	// Create tables
	if err := createTables(); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to create tables: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Cleanup
	dropTables()

	dbSQL.Close()
	dbSqlx.Close()
	sqlDB, _ = dbGorm.DB()
	sqlDB.Close()
	dbXorm.Close()
	dbBun.Close()

	os.Exit(code)
}

func createTables() error {
	var queries []string

	switch dbType {
	case "postgres":
		queries = []string{
			`CREATE TABLE IF NOT EXISTS bench_users (
				id BIGSERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) NOT NULL,
				age INT NOT NULL DEFAULT 0,
				status VARCHAR(50) NOT NULL DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NULL,
				deleted_at TIMESTAMP NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_bench_users_email ON bench_users(email)`,
			`CREATE INDEX IF NOT EXISTS idx_bench_users_status ON bench_users(status)`,

			`CREATE TABLE IF NOT EXISTS bench_products (
				id BIGSERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				price DECIMAL(10,2) NOT NULL DEFAULT 0,
				category VARCHAR(100) NOT NULL DEFAULT '',
				stock INT NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NULL,
				deleted_at TIMESTAMP NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_bench_products_category ON bench_products(category)`,

			`CREATE TABLE IF NOT EXISTS bench_orders (
				id BIGSERIAL PRIMARY KEY,
				user_id BIGINT NOT NULL,
				total DECIMAL(12,2) NOT NULL DEFAULT 0,
				status VARCHAR(50) NOT NULL DEFAULT 'pending',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NULL,
				deleted_at TIMESTAMP NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_bench_orders_user_id ON bench_orders(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_bench_orders_status ON bench_orders(status)`,
		}
	default: // mysql
		queries = []string{
			`CREATE TABLE IF NOT EXISTS bench_users (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) NOT NULL,
				age INT NOT NULL DEFAULT 0,
				status VARCHAR(50) NOT NULL DEFAULT 'active',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NULL,
				deleted_at DATETIME NULL,
				INDEX idx_email (email),
				INDEX idx_status (status)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

			`CREATE TABLE IF NOT EXISTS bench_products (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				price DECIMAL(10,2) NOT NULL DEFAULT 0,
				category VARCHAR(100) NOT NULL DEFAULT '',
				stock INT NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NULL,
				deleted_at DATETIME NULL,
				INDEX idx_category (category)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

			`CREATE TABLE IF NOT EXISTS bench_orders (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				user_id BIGINT UNSIGNED NOT NULL,
				total DECIMAL(12,2) NOT NULL DEFAULT 0,
				status VARCHAR(50) NOT NULL DEFAULT 'pending',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NULL,
				deleted_at DATETIME NULL,
				INDEX idx_user_id (user_id),
				INDEX idx_status (status)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		}
	}

	for _, q := range queries {
		if _, err := dbSQL.Exec(q); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}

func dropTables() {
	tables := []string{"bench_orders", "bench_products", "bench_users"}
	for _, t := range tables {
		if dbType == "postgres" {
			dbSQL.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
		} else {
			dbSQL.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
		}
	}
}

func truncateAllTables() {
	tables := []string{"bench_users", "bench_products", "bench_orders"}
	for _, t := range tables {
		if dbType == "postgres" {
			dbSQL.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", t))
		} else {
			dbSQL.Exec(fmt.Sprintf("TRUNCATE TABLE %s", t))
		}
	}
}

func truncateTable(name string) {
	if dbType == "postgres" {
		dbSQL.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", name))
	} else {
		dbSQL.Exec(fmt.Sprintf("TRUNCATE TABLE %s", name))
	}
}

// placeholder returns the SQL placeholder for the given argument position.
// MySQL uses "?" for all positions; PostgreSQL uses "$1", "$2", etc.
func placeholder(pos int) string {
	if dbType == "postgres" {
		return fmt.Sprintf("$%d", pos)
	}
	return "?"
}

// placeholders returns n SQL placeholders separated by ", ".
func placeholders(n, start int) string {
	s := placeholder(start)
	for i := 1; i < n; i++ {
		s += ", " + placeholder(start+i)
	}
	return s
}

// bulkPlaceholders generates (p1,p2,...),(p1,p2,...),... for bulk inserts.
// colsPerRow = number of columns per row, rowCount = number of rows, start = starting position.
func bulkPlaceholders(colsPerRow, rowCount, start int) string {
	result := ""
	for r := 0; r < rowCount; r++ {
		if r > 0 {
			result += ", "
		}
		result += "(" + placeholders(colsPerRow, start+r*colsPerRow) + ")"
	}
	return result
}
