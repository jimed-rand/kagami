.PHONY: build install uninstall clean test fmt lint deps tidy check-prereqs version all help

BINARY_NAME=kagami
VERSION=4.1
INSTALL_PATH=/usr/bin
CONFIG_DIR=/etc/kagami

help:
	@echo "Kagami $(VERSION) - Debian/Ubuntu ISO Builder"
	@echo ""
	@echo "Available targets:"
	@echo "  build         Build the Kagami binary (static/portable)"
	@echo "  install       Install Kagami to system (requires sudo)"
	@echo "  uninstall     Remove Kagami from system (requires sudo)"
	@echo "  clean         Remove build artifacts"
	@echo "  test          Run test suite"
	@echo "  fmt           Format Go source code"
	@echo "  lint          Run static analysis"
	@echo "  deps          Download module dependencies"
	@echo "  tidy          Tidy go.mod and go.sum"
	@echo "  check-prereqs Verify system prerequisites"
	@echo "  version       Display version information"
	@echo "  all           Clean, download deps, and build"

build:
	@echo "[INFO] Building Kagami $(VERSION) (static/portable)..."
	@CGO_ENABLED=0 go build -ldflags="-X kagami/pkg/config.Version=$(VERSION) -s -w" -o $(BINARY_NAME) .
	@echo "[OK] Build complete: ./$(BINARY_NAME)"
	@echo "     Note: This binary requires an APT-based system to execute."

install: build
	@echo "[INFO] Installing Kagami $(VERSION) to $(INSTALL_PATH)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo mkdir -p $(CONFIG_DIR)/examples
	@sudo cp examples/*.json $(CONFIG_DIR)/examples/
	@echo "[OK] Installation complete"
	@echo "     Binary:   $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "     Examples: $(CONFIG_DIR)/examples/"
	@echo ""
	@echo "Run 'sudo kagami --install-deps' to install build dependencies"

uninstall:
	@echo "[INFO] Uninstalling Kagami..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo rm -rf $(CONFIG_DIR)
	@echo "[OK] Uninstallation complete"

clean:
	@echo "[INFO] Removing build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "[OK] Clean complete"

test:
	@echo "[INFO] Executing test suite..."
	@go test ./... -v

fmt:
	@echo "[INFO] Formatting Go source code..."
	@go fmt ./...
	@echo "[OK] Formatting complete"

lint:
	@echo "[INFO] Running static analysis..."
	@go vet ./...
	@echo "[OK] Analysis complete"

deps:
	@echo "[INFO] Downloading module dependencies..."
	@go mod download
	@echo "[OK] Dependencies downloaded"

tidy:
	@echo "[INFO] Tidying module dependencies..."
	@go mod tidy
	@echo "[OK] Module dependencies tidied"

check-prereqs:
	@echo "[INFO] Verifying system compatibility..."
	@if [ ! -f /etc/debian_version ] && [ ! -f /etc/lsb-release ]; then \
		echo "[WARNING] Non-Debian/Ubuntu host environment detected."; \
		echo "          Kagami requires an APT-based system to execute."; \
		echo "[TIP] Use Distrobox (Docker/Podman) to run Kagami on non-APT distributions."; \
		echo "      Map your home directory to the container for workspace access."; \
	else \
		echo "[OK] System is APT-based (compatible)"; \
	fi
	@echo ""
	@echo "Execute 'sudo ./$(BINARY_NAME) --check-deps' to verify runtime dependencies"

version:
	@./$(BINARY_NAME) --version 2>/dev/null || echo "Kagami $(VERSION) (binary not built - run 'make build')"

all: clean deps build
