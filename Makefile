APP_NAME := ghostlink
BUILD_DIR := build
GO := go

.PHONY: build test lint clean run

build:
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/ghostlink

test:
	$(GO) test -v -race ./...

lint:
	$(GO) vet ./...

clean:
	rm -rf $(BUILD_DIR)

run: build
	./$(BUILD_DIR)/$(APP_NAME)

# Cross-compilation
.PHONY: build-all
build-all:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/ghostlink
	GOOS=linux GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./cmd/ghostlink
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/ghostlink
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/ghostlink
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/ghostlink
