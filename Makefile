export LINTER_VERSION ?= 1.52.2

bin:
	@mkdir -p bin

tool-lint: bin
	@test -e ./bin/golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin v${LINTER_VERSION}

lint: tool-lint
	./bin/golangci-lint run -v --timeout 3m0s