# Makefile for vault-env

BINARY_NAME=vault-env
BINARY_PATH=./$(BINARY_NAME)
MAIN_PATH=.

.PHONY: all build clean install uninstall test fmt vet help

all: build

# Build the binary
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Install to system PATH
install: build
	sudo mv $(BINARY_NAME) /usr/local/bin/

# Uninstall from system PATH
uninstall:
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Format Go code
fmt:
	go fmt ./...

# Run Go vet
vet:
	go vet ./...

# Run tests
test:
	go test ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  clean      - Clean build artifacts"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  uninstall  - Remove from /usr/local/bin"
	@echo "  fmt        - Format Go code"
	@echo "  vet        - Run go vet"
	@echo "  test       - Run tests"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  help       - Show this help message"