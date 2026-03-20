set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
	@just --list

fmt:
	gofmt -w $(git ls-files '*.go')

fmt-check:
	if [ -n "$(gofmt -l $(git ls-files '*.go'))" ]; then \
		gofmt -l $(git ls-files '*.go'); \
		echo "Run `just fmt` to fix formatting."; \
		exit 1; \
	fi

lint:
	if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; falling back to go vet"; \
		go vet ./...; \
	fi

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build ./...

run:
	go run ./cmd/lazycron

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	go clean ./...
	rm -f coverage.out

mod-tidy:
	go mod tidy
	go mod verify

check:
	just fmt-check
	just lint
	just test

ci:
	just check
	just build
