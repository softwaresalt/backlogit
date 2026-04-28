.PHONY: all build test lint vet fmt cover clean install docs verify-plugin

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

verify-plugin: ## Check plugin copies are in sync with .github/ sources
	@echo "Checking plugin agent copies..."
	@diff plugin/agents/stage.agent.md .github/agents/stage.agent.md \
		|| { echo "DRIFT: stage.agent.md out of date — run: cp .github/agents/stage.agent.md plugin/agents/"; exit 1; }
	@diff plugin/agents/ship.agent.md .github/agents/ship.agent.md \
		|| { echo "DRIFT: ship.agent.md out of date — run: cp .github/agents/ship.agent.md plugin/agents/"; exit 1; }
	@echo "Checking plugin skill copies..."
	@for skill in build-feature compact-context compound compound-refresh deliberate \
	              file-lock fix-ci harness-architect harvest impl-plan \
	              operational-closure plan-harden plan-review pr-lifecycle review \
	              runtime-verification safety-modes skill-search spike; do \
		diff "plugin/skills/$$skill/SKILL.md" ".github/skills/$$skill/SKILL.md" \
			|| { echo "DRIFT: $$skill/SKILL.md out of date"; exit 1; }; \
	done
	@echo "OK: all plugin copies match .github/ sources"
