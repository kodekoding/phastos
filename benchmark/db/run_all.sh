#!/bin/bash
# Run benchmarks on BOTH MySQL and PostgreSQL, then compare
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          DUAL-DATABASE ORM BENCHMARK SUITE                  ║"
echo "║               MySQL + PostgreSQL                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

echo ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
echo "  Phase 1: MySQL Benchmarks"
echo "<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<"
echo ""
"$SCRIPT_DIR/run.sh" --db mysql

echo ""
echo ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
echo "  Phase 2: PostgreSQL Benchmarks"
echo "<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<"
echo ""
"$SCRIPT_DIR/run.sh" --db postgres

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    ALL BENCHMARKS DONE                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "MySQL results:      /tmp/phastos_bench_mysql_results.txt"
echo "PostgreSQL results: /tmp/phastos_bench_postgres_results.txt"
echo ""
echo "Quick comparison:"
echo ""
echo "--- MySQL Insert ---"
grep "BenchmarkStdlib_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkSqlx_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkGorm_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkXorm_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkBeego_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkBun_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
grep "BenchmarkPhastos_Insert\b" /tmp/phastos_bench_mysql_results.txt | head -1
echo ""
echo "--- PostgreSQL Insert ---"
grep "BenchmarkStdlib_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkSqlx_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkGorm_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkXorm_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkBeego_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkBun_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
grep "BenchmarkPhastos_Insert\b" /tmp/phastos_bench_postgres_results.txt | head -1
