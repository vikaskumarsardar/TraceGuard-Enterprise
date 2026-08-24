.PHONY: all build run test clean docker-build

APP_NAME=traceguard
BUILD_DIR=bin

all: build

build:
	@echo "Building TraceGuard Enterprise binary..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/traceguard

run: build
	@echo "Starting TraceGuard Enterprise locally..."
	./$(BUILD_DIR)/$(APP_NAME)

test:
	@echo "Running test suite..."
	go test -v ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

docker-build:
	@echo "Building Docker image..."
	docker build -t traceguard:latest .
