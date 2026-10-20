.PHONY: all build test lint fmt vet tidy clean get web-build web-test

BINARY := noto
CMD     := ./cmd/noto
OUT     := ./bin/$(BINARY)

all: build

## build: compile the binary
build:
	go build -o $(OUT) $(CMD)

## run: run the binary directly
run:
	go run $(CMD)

## test: run all tests
test:
	go test ./...

## web-build: rebuild embedded web assets (requires Bun)
web-build:
	cd internal/web/ui && bun install --frozen-lockfile && bun run build

## web-test: run web UI state tests (requires Bun)
web-test:
	cd internal/web/ui && bun test state.test.js

## test-integration: run integration tests
test-integration:
	go test ./tests/integration/...

## test-contract: run contract tests
test-contract:
	go test ./tests/contract/...

## test-unit: run unit tests
test-unit:
	go test ./tests/unit/...

## bench: run benchmark tests
bench:
	go test -bench=. -benchmem ./tests/integration/...

## lint: run golangci-lint
lint:
	~/go/bin/golangci-lint run ./...

## fmt: format all Go source
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## clean: remove build artefacts
clean:
	rm -rf ./bin

### get: download dependencies
get:
	go get -u ./...
