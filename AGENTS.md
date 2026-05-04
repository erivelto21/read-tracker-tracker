# Copilot Coding Agent Instructions

This document provides instructions for AI coding agents working on this repository.

## Project Overview

This is a Golang/Gin implementation of an application designed to track your reading progress across multiple literary works.

## Technology Stack

| Layer | Technology | Version |
| :--- | :--- | :--- |
| **Language** | [Go](https://go.dev/) | `1.25` |
| **Web Framework** | [Gin Gonic](https://github.com/gin-gonic/gin) | `v1.12.0` |
| **Database Driver** | [Mongo Go Driver](https://github.com/mongodb/mongo-go-driver) | `v2.0` |
| **Validation** | [Go Playground Validator](https://github.com/go-playground/validator) | `v10.0` |

## Directory Structure

```
tracker/
├── cmd/                # Entry points (main.go files)
│   └── api/            # Gin server entry point
│       └── main.go     
├── handler/            # Gin route handlers
├── service/            # Business logic
├── repository/         # Data access
├── domain/             # Structs/Models
├── config/             # Configuration loading
├── api/                # API definitions (OpenAPI/Swagger specs)
├── go.mod              # Module definition
└── go.sum              # Dependency checksums
```

## Development Commands

```bash
# Install dependencies
go mod download

# Build
go build ./...

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Format code
go fmt ./...

# Run linter
golangci-lint run

# Start the server
go run cmd/api/main.go
```
