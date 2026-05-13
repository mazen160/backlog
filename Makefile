BIN        := backlog
BUILD_DIR  := .
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test test-verbose test-race test-e2e fmt vet lint cover install clean snapshot tidy

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BIN) ./cmd/backlog

install:
	go install $(LDFLAGS) ./cmd/backlog

test:
	go test ./... -timeout 120s

test-race:
	go test -race ./... -timeout 120s

test-verbose:
	go test ./... -v -timeout 120s

test-e2e: build
	cd e2e && npx playwright test

tidy:
	go mod tidy

fmt:
	gofmt -w ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -f $(BIN) coverage.out
	rm -rf dist/

.DEFAULT_GOAL := build
