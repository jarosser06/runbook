.PHONY: all build test lint clean deps install run

# Default target - run everything
all: lint test build

# Build binary to bin/
build:
	@echo "Building dev-toolkit-mcp..."
	@mkdir -p bin
	go build -o bin/dev-toolkit-mcp main.go
	@echo "Built: bin/dev-toolkit-mcp"

# Run tests with race detection and coverage
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "\nCoverage summary:"
	go tool cover -func=coverage.out

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out
	@echo "Cleaned bin/ and coverage files"

# Install dev dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Dependencies installed"

# Install binary to $HOME/.bin/
install: build
	@echo "Installing binary..."
	@mkdir -p $(HOME)/.bin
	cp bin/dev-toolkit-mcp $(HOME)/.bin/
	@echo "Installed to $(HOME)/.bin/dev-toolkit-mcp"

# Run the server (for testing)
run: build
	./bin/dev-toolkit-mcp
