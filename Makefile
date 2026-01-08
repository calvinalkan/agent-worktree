SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables --no-builtin-rules -j
.SUFFIXES:
.DELETE_ON_ERROR:
.DEFAULT_GOAL := build

.PHONY: build test lint clean install vet install-tools check modernize fmt

BINARY := wt
GO := go
VERSION := dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")$(shell git diff --quiet 2>/dev/null || echo "-dirty")
DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/wt
	@[ -e ~/.local/bin/$(BINARY) ] || ln -sf $(CURDIR)/$(BINARY) ~/.local/bin/$(BINARY)

modernize:
	modernize -fix ./...

vet:
	$(GO) vet ./...

fmt: modernize
	golangci-lint fmt

lint:
	golangci-lint config verify
	@./backpressure/no-lint-suppress.sh
	golangci-lint run --fix ./...

test:
	$(GO) test -race ./...

clean:
	rm -f $(BINARY)

install:
	$(GO) install ./cmd/wt

install-tools:
	$(GO) install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest

check: vet lint test
