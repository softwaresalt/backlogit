.PHONY: all build test lint vet fmt cover clean install docs docs-lint verify-plugin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
LDFLAGS := -X github.com/softwaresalt/backlogit/internal/version.Version=$(VERSION) \
           -X github.com/softwaresalt/backlogit/internal/version.Commit=$(COMMIT) \
           -X github.com/softwaresalt/backlogit/internal/version.BuildDate=$(DATE)

all: fmt vet lint test build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/backlogit ./cmd/backlogit

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	@bad=$$(gofmt -l .); test -z "$$bad" || { printf '%s\n' "$$bad"; exit 1; }

cover:
	go tool cover -func=coverage.out

clean:
	rm -rf bin/ coverage.out

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/backlogit

docs:
	go run ./cmd/gen-docs docs/cli-reference

docs-lint: ## Enforce docline frontmatter compliance on authored docs
	go run ./cmd/backlogit docs lint

verify-plugin: ## Check plugin bundle structure against its manifest
	go test ./tests/integration/ -run 'TestPluginBundleStructurallyValid' -count=1
