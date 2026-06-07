.PHONY: build test test-integration vet fmt clean tidy lint vulncheck

BIN := bin/pinax
PKG := ./...

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X pinax/internal/buildinfo.Version=$(VERSION) \
	-X pinax/internal/buildinfo.Commit=$(COMMIT) \
	-X pinax/internal/buildinfo.Date=$(DATE)

build:
	@mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/pinax

test:
	go test -race -timeout 120s $(PKG)

test-integration:
	go test -tags integration -timeout 10m $(PKG)

vet:
	go vet $(PKG)

lint:
	golangci-lint run

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest $(PKG)

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

clean:
	rm -rf bin coverage.out
