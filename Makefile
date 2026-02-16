.PHONY: build install clean test help check-prereqs

BINARY_NAME=kagami
VERSION=4.0
INSTALL_PATH=/usr/bin
CONFIG_DIR=/etc/kagami

help: ## Show this help message
	@echo "Kagami $(VERSION) - Ubuntu ISO Builder"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the Kagami binary
	@echo "[STEP] Building Kagami $(VERSION)..."
	@go build -ldflags="-X kagami/pkg/config.Version=$(VERSION)" -o $(BINARY_NAME) .
	@echo "[SUCCESS] Build complete: ./$(BINARY_NAME)"

install: build ## Install Kagami to system (requires sudo)
	@echo "[STEP] Installing Kagami $(VERSION) to $(INSTALL_PATH)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo mkdir -p $(CONFIG_DIR)/examples
	@sudo cp examples/*.json $(CONFIG_DIR)/examples/
	@echo "[SUCCESS] Installation complete"
	@echo "  Binary: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "  Examples: $(CONFIG_DIR)/examples/"
	@echo ""
	@echo "Run 'sudo kagami --install-deps' to install dependencies"

uninstall: ## Uninstall Kagami from system (requires sudo)
	@echo "[ACTION] Uninstalling Kagami..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo rm -rf $(CONFIG_DIR)
	@echo "[SUCCESS] Uninstalled"

clean: ## Remove build artifacts
	@echo "[ACTION] Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "[SUCCESS] Clean complete"

test: ## Run tests
	@echo "[STEP] Running tests..."
	@go test ./... -v

fmt: ## Format Go code
	@echo "[ACTION] Formatting code..."
	@go fmt ./...
	@echo "[SUCCESS] Code formatted"

lint: ## Run linter
	@echo "[ACTION] Running linter..."
	@go vet ./...
	@echo "[SUCCESS] Lint complete"

deps: ## Download dependencies
	@echo "[ACTION] Downloading dependencies..."
	@go mod download
	@echo "[SUCCESS] Dependencies downloaded"

tidy: ## Tidy go.mod
	@echo "[ACTION] Tidying dependencies..."
	@go mod tidy
	@echo "[SUCCESS] Dependencies tidied"

check-prereqs: ## Check system prerequisites
	@echo "[INFO] Checking system compatibility..."
	@command -v apt-get >/dev/null 2>&1 || { echo "[ERROR] APT not found - Kagami requires Ubuntu/Debian"; exit 1; }
	@command -v dpkg >/dev/null 2>&1 || { echo "[ERROR] dpkg not found - Kagami requires Ubuntu/Debian"; exit 1; }
	@test -f /etc/debian_version -o -f /etc/lsb-release || { echo "[ERROR] Not a Debian/Ubuntu system"; exit 1; }
	@echo "[SUCCESS] System is APT-based (compatible)"
	@echo ""
	@echo "Run 'sudo ./$(BINARY_NAME) --check-deps' to verify build dependencies"

version: ## Show version information
	@./$(BINARY_NAME) --version 2>/dev/null || echo "Kagami $(VERSION) (not built yet - run 'make build')"

all: clean deps build ## Clean, download deps and build

test-deps: ## Test dependency checking
	@sudo ./$(BINARY_NAME) --check-deps || true

install-deps: build ## Install system dependencies
	@echo "[STEP] Installing system dependencies..."
	@sudo ./$(BINARY_NAME) --install-deps
