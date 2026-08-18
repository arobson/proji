.DEFAULT_GOAL := help

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*##"}{printf "  %-16s %s\n", $$1, $$2}'

build: ## Build the proji binary into bin/
	go build -o bin/proji ./cmd/proji

install: ## go install proji into GOBIN
	go install ./cmd/proji

test: ## Run unit tests (fast; excludes integration tests)
	go test ./... -race -covermode=atomic -coverprofile=coverage.out

test-integration: ## Run tests that shell out to a real git binary
	go test -tags=integration ./... -run Integration -v

cover: test ## Show per-function coverage
	go tool cover -func=coverage.out

fmt: ## Format code
	gofmt -w .

fmt-check: ## Fail if code is not gofmt-formatted
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

vet: ## go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

sec: ## Run gosec
	go run github.com/securego/gosec/v2/cmd/gosec@latest ./...

vuln: ## Run govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

tidy: ## go mod tidy
	go mod tidy

tidy-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy && git diff --exit-code go.mod go.sum

clean: ## Remove build artifacts
	rm -rf bin coverage.out

pre-commit: fmt-check vet lint sec vuln tidy-check test test-integration build ## Run everything CI runs

.PHONY: help build install test test-integration cover fmt fmt-check vet lint sec vuln tidy tidy-check clean pre-commit
