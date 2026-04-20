.PHONY: all build test lint vet fmt cover clean install docs

all: fmt vet lint test build

build:
	go build -o bin/backlogit ./cmd/backlogit

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
	go install ./cmd/backlogit

docs:
	go run ./cmd/gen-docs docs/cli-reference
