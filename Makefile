BINARY_NAME=chirpy
BUILD_DIR=bin

all: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"

build: ## Build the Go binary
	@echo "Building..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/chirpy/main.go

clean: ## Remove build binaries
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)

run : ## Build and Run go program
	@./$(BUILD_DIR)/$(BINARY_NAME)

