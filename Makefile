.PHONY: all build test lint fmt tidy vulncheck clean

# Toolchain wrapper. Go is managed by `mise` (see mise.toml). If `go` is
# already on PATH (mise shell activation done), use bare invocations;
# otherwise prefix with `mise exec --` so Make can find the toolchain.
ifneq (,$(shell command -v go 2>/dev/null))
    EXEC :=
else
    EXEC := mise exec --
endif

all: tidy fmt lint test build

build:
	$(EXEC) go build ./...
	$(EXEC) go build -o forbidcalls ./cmd/forbidcalls

test:
	$(EXEC) go test ./...

lint:
	$(EXEC) golangci-lint run ./...

fmt:
	$(EXEC) gofumpt -w .
	$(EXEC) goimports -w . 2>/dev/null || true

tidy:
	$(EXEC) go mod tidy

vulncheck:
	$(EXEC) govulncheck ./...

clean:
	rm -f forbidcalls
	rm -f coverage.out coverage.html
