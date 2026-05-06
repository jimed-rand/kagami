.PHONY: help build install uninstall

GO ?= go
BINARY_NAME ?= kagami
BINARY_PATH ?= ./$(BINARY_NAME)
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
PKG ?= ./cmd/kagami
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

build:
	@echo "==> Building $(BINARY_NAME) (version: $(VERSION))"
	$(GO) build -ldflags "$(LDFLAGS)" -o "$(BINARY_PATH)" "$(PKG)"
	@echo "==> Build complete: $(BINARY_PATH)"

install: build
	@echo "==> Installing $(BINARY_NAME) to $(BINDIR)"
	install -d "$(BINDIR)"
	install -m 0755 "$(BINARY_PATH)" "$(BINDIR)/$(BINARY_NAME)"
	@echo "==> Install complete: $(BINDIR)/$(BINARY_NAME)"

uninstall:
	@echo "==> Uninstalling $(BINARY_NAME) from $(BINDIR)"
	rm -f "$(BINDIR)/$(BINARY_NAME)"
	@echo "==> Uninstall complete"
