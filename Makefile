.PHONY: all build test lint clean deps install run

# Default target - run everything
all: lint test build

# Build binary to bin/
build:
	@echo "Building runbook..."
	@mkdir -p bin
	go build -o bin/runbook main.go
	@echo "Built: bin/runbook"

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
	cp bin/runbook $(HOME)/.bin/
	@echo "Installed to $(HOME)/.bin/runbook"

# Run the server (for testing)
run: build
	./bin/runbook
