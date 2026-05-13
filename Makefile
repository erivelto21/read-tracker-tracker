.PHONY: help setup run build test test-race fmt lint tidy swag

BINARY_NAME=tracker
CMD_PATH=./cmd/api

ifneq (,$(wildcard ../.env))
include ../.env
export
endif

## help: show available commands
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'

## setup: install all required dev tools
setup:
	@echo "Installing dev tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go mod download
	@echo "Done. You're ready to go."

## run: start the API server
run:
	go run $(CMD_PATH)/main.go

## build: compile the binary
build:
	go build -o bin/$(BINARY_NAME) $(CMD_PATH)/main.go

## test: run all tests
test:
	go test ./...

## test-race: run all tests with race condition detector
test-race:
	go test -race ./...

## test-cover: run tests with coverage report
test-cover:
	go test -cover ./...

## fmt: format code with gofmt and goimports
fmt:
	go fmt ./...
	goimports -w .

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## swag: regenerate Swagger docs from annotations
swag:
	swag init -g cmd/api/main.go
