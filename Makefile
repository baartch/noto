.PHONY: all build test lint fmt vet tidy clean

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
	golangci-lint run ./...

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
