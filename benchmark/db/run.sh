#!/bin/bash
# Phastos ORM Benchmark Runner
# Compares: database/sql, sqlx, GORM, XORM, Beego ORM, Bun, Phastos ORM
# 
# Usage:
#   ./run.sh [--db mysql|postgres] [DSN]
#   ./run.sh --db mysql
#   ./run.sh --db postgres
#   ./run.sh --db mysql "root:pass@tcp(127.0.0.1:3306)/phastos_benchmark?parseTime=true"
#
# Default: --db mysql

set -e

DB_TYPE="mysql"
CUSTOM_DSN=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --db)
            DB_TYPE="$2"
            shift 2
            ;;
        *)
            CUSTOM_DSN="$1"
            shift
            ;;
    esac
done

# Set DSN
if [ -n "$CUSTOM_DSN" ]; then
    DSN="$CUSTOM_DSN"
else
    case $DB_TYPE in
        postgres)
            DSN="postgres://postgres:postgres@127.0.0.1:32771/phastos_benchmark?sslmode=disable"
            ;;
        mysql|*)
            DSN="root:toor@tcp(127.0.0.1:3306)/phastos_benchmark?parseTime=true&multiStatements=true"
            ;;
    esac
fi

export BENCHMARK_DB_TYPE="$DB_TYPE"
export BENCHMARK_DB_DSN="$DSN"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Phastos vs GORM vs XORM vs Beego vs Bun vs sqlx vs sql    ║"
echo "║                    BENCHMARK SUITE                          ║"
echo "║                   [${DB_TYPE^^}]                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "DB Type: $DB_TYPE"
echo "DSN:     $DSN"
echo ""

# Create database if needed
case $DB_TYPE in
    postgres)
        PGPASSWORD=postgres psql -h 127.0.0.1 -p 32771 -U postgres -c "CREATE DATABASE phastos_benchmark;" 2>/dev/null || echo "Note: Could not auto-create DB, assuming it exists"
        ;;
    mysql|*)
        docker exec mysql_container mysql -u root -ptoor -e "CREATE DATABASE IF NOT EXISTS phastos_benchmark;" 2>/dev/null || echo "Note: Could not auto-create DB, assuming it exists"
        ;;
esac

echo "Running benchmarks..."
echo ""

cd "$SCRIPT_DIR"

# Run benchmarks from the db module (separate go.mod)
go test -bench=. -benchmem -benchtime=3s -timeout=30m ./... \
    -run=^$ \
    2>&1 | tee "/tmp/phastos_bench_${DB_TYPE}_results.txt"

echo ""
echo "══════════════════════════════════════════════════════════════"
echo "Benchmark complete! Results saved to /tmp/phastos_bench_${DB_TYPE}_results.txt"
echo "══════════════════════════════════════════════════════════════"
