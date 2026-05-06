.PHONY: help build install uninstall run clean fmt vet test version

GO ?= go
BINARY_NAME ?= kagami
BINARY_PATH ?= ./$(BINARY_NAME)
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
PKG ?= ./cmd/kagami
ARGS ?=
GIT_COMMIT_COUNT := $(shell git rev-list --count HEAD 2>/dev/null || echo 0)
GIT_SHA := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo nogit)
GIT_DIRTY := $(shell if [ -n "$$(git status --porcelain 2>/dev/null)" ]; then echo -dirty; fi)
VERSION ?= rolling.$(GIT_COMMIT_COUNT).$(GIT_SHA)$(GIT_DIRTY)
LDFLAGS ?= -X main.version=$(VERSION)

help:
	@echo "Kagami build helpers"
	@echo
	@echo "Targets:"
	@echo "  make build      Build binary at $(BINARY_PATH)"
	@echo "  make install    Install binary to $(BINDIR)"
	@echo "  make uninstall  Remove installed binary from $(BINDIR)"
	@echo "  make run        Run with go run $(PKG) (pass ARGS=\"...\")"
	@echo "  make version    Print resolved build version"
	@echo "  make clean      Remove local build artifacts"
	@echo "  make fmt        Run gofmt on all Go files"
	@echo "  make vet        Run go vet ./..."
	@echo "  make test       Run go test ./..."
	@echo
	@echo "Variables (override with VAR=value):"
	@echo "  GO=$(GO)"
	@echo "  BINARY_NAME=$(BINARY_NAME)"
	@echo "  BINARY_PATH=$(BINARY_PATH)"
	@echo "  PREFIX=$(PREFIX)"
	@echo "  BINDIR=$(BINDIR)"
	@echo "  PKG=$(PKG)"
	@echo "  VERSION=$(VERSION)"
	@echo "  LDFLAGS=$(LDFLAGS)"
	@echo "  ARGS=$(ARGS)"

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o "$(BINARY_PATH)" "$(PKG)"

install: build
	install -d "$(BINDIR)"
	install -m 0755 "$(BINARY_PATH)" "$(BINDIR)/$(BINARY_NAME)"

uninstall:
	rm -f "$(BINDIR)/$(BINARY_NAME)"

run:
	$(GO) run -ldflags "$(LDFLAGS)" "$(PKG)" $(ARGS)

version:
	@echo "$(VERSION)"

clean:
	rm -f "./$(BINARY_NAME)"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...
