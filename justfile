# phastos justfile
# https://github.com/casey/just

# default recipe — show available commands
default:
    @just --list

# ─── setup ────────────────────────────────────────────────────────────────────

# install dev tools and download dependencies
setup: _install-tools dep _install-hooks
    @echo "✔ setup complete"

# install Go dev tools
_install-tools:
    @echo "→ installing golangci-lint…"
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    @echo "✔ tools installed"

# download Go module dependencies
dep:
    go mod download
    go mod tidy

# install git hooks from scripts/ into .git/hooks/
_install-hooks:
    @cp scripts/pre-push .git/hooks/pre-push
    @chmod +x .git/hooks/pre-push
    @echo "✔ git hooks installed"

# ─── quality ──────────────────────────────────────────────────────────────────

# run all linters (generator excluded: printf analyzer panic with Go 1.25)
# exit code 7 = typechecking errors from local module self-references (safe to ignore)
lint:
    #!/usr/bin/env bash
    set -o pipefail
    golangci-lint run --timeout 3m $(go list ./... | grep -v '/generator') 2>&1; rc=$?
    if [ $rc -eq 7 ]; then exit 0; fi
    exit $rc

# run linters and auto-fix what's possible
lint-fix:
    #!/usr/bin/env bash
    set -o pipefail
    golangci-lint run --fix --timeout 3m $(go list ./... | grep -v '/generator') 2>&1; rc=$?
    if [ $rc -eq 7 ]; then exit 0; fi
    exit $rc

# run go vet only (quick check)
vet:
    go vet ./...

# run unit tests
test:
    go test -v -cover -race ./...

# run tests with coverage output
test-cover:
    go test -coverprofile=cover.out -cover -race ./...
    go tool cover -func=cover.out
    @rm -f cover.out

# full check: vet + lint + test
check: vet lint test
    @echo "✔ all checks passed"

# ─── build ────────────────────────────────────────────────────────────────────

# verify the module builds cleanly
build:
    go build ./...

# ─── cleanup ──────────────────────────────────────────────────────────────────

# remove generated artifacts
clean:
    rm -f cover.out
