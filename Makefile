# ============================================================================
# SiloBang - Unified Build System
# ============================================================================

# Variables
FRONTEND_DIR := web-src
BUILD_DIR := web/dist
BINARY_NAME := silobang

# Go variables
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build

# Node variables
NPM := npm
NPM_INSTALL := $(NPM) install
NPM_BUILD := $(NPM) run build
NPM_DEV := $(NPM) run dev

# Colors for output
COLOR_RESET := \033[0m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

# Phony targets
.PHONY: help install \
        dev dev-frontend dev-backend \
        build build-frontend build-backend build-all \
        test test-verbose test-coverage \
        clean clean-frontend clean-backend \
        run run-production \
        fmt vet lint \
        release release-status release-clean

# ============================================================================
# Default Target
# ============================================================================

help:
	@echo "$(COLOR_BLUE)SiloBang - Build System$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)Setup:$(COLOR_RESET)"
	@echo "  make install         - Install npm dependencies"
	@echo ""
	@echo "$(COLOR_GREEN)Development:$(COLOR_RESET)"
	@echo "  make dev             - Show instructions for running both servers"
	@echo "  make dev-frontend    - Start frontend dev server (port 5173)"
	@echo "  make dev-backend     - Run Go backend (port 2369)"
	@echo ""
	@echo "$(COLOR_GREEN)Build:$(COLOR_RESET)"
	@echo "  make build           - Build complete application (frontend + backend)"
	@echo "  make build-frontend  - Build frontend for production"
	@echo "  make build-backend   - Build Go binary with embedded frontend"
	@echo ""
	@echo "$(COLOR_GREEN)Test:$(COLOR_RESET)"
	@echo "  make test            - Run Go tests"
	@echo "  make test-verbose    - Run Go tests with verbose output"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo ""
	@echo "$(COLOR_GREEN)Run:$(COLOR_RESET)"
	@echo "  make run             - Build and run application"
	@echo "  make run-production  - Run built binary (must build first)"
	@echo ""
	@echo "$(COLOR_GREEN)Release:$(COLOR_RESET)"
	@echo "  make release         - Tag and push to trigger CI release"
	@echo "  make release-status  - Check release workflow status"
	@echo "  make release-clean   - Clean up failed release (tag + artifacts)"
	@echo ""
	@echo "$(COLOR_GREEN)Maintenance:$(COLOR_RESET)"
	@echo "  make clean           - Remove all build artifacts"
	@echo "  make clean-frontend  - Remove frontend build artifacts"
	@echo "  make clean-backend   - Remove Go binary"
	@echo "  make fmt             - Format Go code"
	@echo "  make vet             - Run Go vet"
	@echo ""
	@echo "$(COLOR_YELLOW)First time setup:$(COLOR_RESET)"
	@echo "  make install && make build"

# ============================================================================
# Setup Targets
# ============================================================================

install:
	@echo "$(COLOR_BLUE)Installing npm dependencies...$(COLOR_RESET)"
	@cd $(FRONTEND_DIR) && $(NPM_INSTALL)
	@echo "$(COLOR_GREEN)✓ Dependencies installed$(COLOR_RESET)"

# ============================================================================
# Development Targets
# ============================================================================

dev:
	@echo "$(COLOR_BLUE)SiloBang Development Mode$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_YELLOW)Run these commands in separate terminals:$(COLOR_RESET)"
	@echo ""
	@echo "  $(COLOR_GREEN)Terminal 1 (Backend):$(COLOR_RESET)"
	@echo "    make dev-backend"
	@echo ""
	@echo "  $(COLOR_GREEN)Terminal 2 (Frontend):$(COLOR_RESET)"
	@echo "    make dev-frontend"
	@echo ""
	@echo "The frontend will proxy API calls to the backend on port 2369."
	@echo "Access the application at: $(COLOR_BLUE)http://localhost:5173$(COLOR_RESET)"
	@echo ""

dev-frontend:
	@echo "$(COLOR_BLUE)Starting frontend dev server...$(COLOR_RESET)"
	@cd $(FRONTEND_DIR) && $(NPM_DEV)

dev-backend:
	@echo "$(COLOR_BLUE)Starting Go backend...$(COLOR_RESET)"
	@$(GO) run ./cmd/silobang

# ============================================================================
# Build Targets
# ============================================================================

build: build-frontend build-backend
	@echo "$(COLOR_GREEN)✓ Build complete!$(COLOR_RESET)"
	@echo "Run the application with: ./$(BINARY_NAME)"

build-all: build

build-frontend:
	@echo "$(COLOR_BLUE)Building frontend...$(COLOR_RESET)"
	@cd $(FRONTEND_DIR) && $(NPM_BUILD)
	@echo "$(COLOR_GREEN)✓ Frontend built → $(BUILD_DIR)$(COLOR_RESET)"

build-backend: build-frontend
	@echo "$(COLOR_BLUE)Building Go binary...$(COLOR_RESET)"
	@$(GOBUILD) $(GOFLAGS) -o $(BINARY_NAME) ./cmd/silobang
	@echo "$(COLOR_GREEN)✓ Backend built → ./$(BINARY_NAME)$(COLOR_RESET)"

# ============================================================================
# Test Targets
# ============================================================================

test:
	@echo "$(COLOR_BLUE)Running Go tests...$(COLOR_RESET)"
	@$(GOTEST) ./...

test-verbose:
	@echo "$(COLOR_BLUE)Running Go tests (verbose)...$(COLOR_RESET)"
	@$(GOTEST) -v ./...

test-coverage:
	@echo "$(COLOR_BLUE)Running Go tests with coverage...$(COLOR_RESET)"
	@$(GOTEST) -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report generated → coverage.html$(COLOR_RESET)"

# ============================================================================
# Run Targets
# ============================================================================

run: build
	@echo "$(COLOR_BLUE)Starting SiloBang...$(COLOR_RESET)"
	@./$(BINARY_NAME)

run-production:
	@if [ ! -f ./$(BINARY_NAME) ]; then \
		echo "$(COLOR_YELLOW)Error: Binary not found. Run 'make build' first.$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_BLUE)Starting SiloBang (production)...$(COLOR_RESET)"
	@./$(BINARY_NAME)

# ============================================================================
# Clean Targets
# ============================================================================

clean: clean-frontend clean-backend
	@echo "$(COLOR_GREEN)✓ All build artifacts removed$(COLOR_RESET)"

clean-frontend:
	@echo "$(COLOR_BLUE)Cleaning frontend...$(COLOR_RESET)"
	@rm -rf $(FRONTEND_DIR)/node_modules
	@rm -rf $(BUILD_DIR)
	@echo "$(COLOR_GREEN)✓ Frontend cleaned$(COLOR_RESET)"

clean-backend:
	@echo "$(COLOR_BLUE)Cleaning backend...$(COLOR_RESET)"
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@echo "$(COLOR_GREEN)✓ Backend cleaned$(COLOR_RESET)"

# ============================================================================
# Utility Targets
# ============================================================================

fmt:
	@echo "$(COLOR_BLUE)Formatting Go code...$(COLOR_RESET)"
	@$(GO) fmt ./...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

vet:
	@echo "$(COLOR_BLUE)Running Go vet...$(COLOR_RESET)"
	@$(GO) vet ./...
	@echo "$(COLOR_GREEN)✓ Go vet passed$(COLOR_RESET)"

lint:
	@echo "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not found. Install from: https://golangci-lint.run/$(COLOR_RESET)"; \
	fi

# ============================================================================
# Release Targets
# ============================================================================

release:
	@bash scripts/release.sh

release-status:
	@bash scripts/release-status.sh

release-clean:
	@bash scripts/release-clean.sh
