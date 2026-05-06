.PHONY: help build install uninstall run clean fmt vet test

GO ?= go
BINARY_NAME ?= kagami
BIN_DIR ?= bin
BINARY_PATH ?= $(BIN_DIR)/$(BINARY_NAME)
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
PKG ?= ./cmd/kagami
ARGS ?=

help:
	@echo "Kagami build helpers"
	@echo
	@echo "Targets:"
	@echo "  make build      Build binary at $(BINARY_PATH)"
	@echo "  make install    Install binary to $(BINDIR)"
	@echo "  make uninstall  Remove installed binary from $(BINDIR)"
	@echo "  make run        Run with go run $(PKG) (pass ARGS=\"...\")"
	@echo "  make clean      Remove local build artifacts"
	@echo "  make fmt        Run gofmt on all Go files"
	@echo "  make vet        Run go vet ./..."
	@echo "  make test       Run go test ./..."
	@echo
	@echo "Variables (override with VAR=value):"
	@echo "  GO=$(GO)"
	@echo "  BINARY_NAME=$(BINARY_NAME)"
	@echo "  BIN_DIR=$(BIN_DIR)"
	@echo "  PREFIX=$(PREFIX)"
	@echo "  BINDIR=$(BINDIR)"
	@echo "  PKG=$(PKG)"
	@echo "  ARGS=$(ARGS)"

build:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(BINARY_PATH)" "$(PKG)"

install: build
	install -d "$(BINDIR)"
	install -m 0755 "$(BINARY_PATH)" "$(BINDIR)/$(BINARY_NAME)"

uninstall:
	rm -f "$(BINDIR)/$(BINARY_NAME)"

run:
	$(GO) run "$(PKG)" $(ARGS)

clean:
	rm -rf "$(BIN_DIR)"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...
