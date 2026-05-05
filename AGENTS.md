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

## Getting Started

After cloning, run the one-time setup to install all required dev tools:

```bash
make setup
```

## Development Commands

| Command | Description |
| :--- | :--- |
| `make setup` | Install dev tools (`goimports`, `golangci-lint`) and download dependencies |
| `make run` | Start the API server |
| `make build` | Compile the binary to `bin/tracker` |
| `make test` | Run all tests |
| `make test-race` | Run all tests with the race detector |
| `make test-cover` | Run tests with coverage report |
| `make fmt` | Format code with `go fmt` and `goimports` |
| `make lint` | Run `golangci-lint` |
| `make tidy` | Tidy and verify go modules |
| `make help` | List all available targets |

## Code Guidelines

### 1. Error Handling

- Always handle errors explicitly. **Never ignore errors with `_`**.
- Return errors as the **last return value**.
- Wrap errors with context using `fmt.Errorf("context: %w", err)` so the call chain is traceable.
- Use `errors.Is` and `errors.As` for comparison and type assertion — never compare error strings.
- Define sentinel errors in the `domain` package for domain-level failures (e.g., `ErrNotFound`, `ErrInvalidInput`).
- Avoid `panic` for expected errors. Use it only for truly unrecoverable situations (e.g., missing required config at startup).

```go
// Good
user, err := s.repo.FindByID(ctx, id)
if err != nil {
    return fmt.Errorf("service.FindUser: %w", err)
}

// Bad
user, _ := s.repo.FindByID(ctx, id)
```

---

### 2. Code Style & Formatting

- Run `go fmt ./...` before every commit. CI must enforce this.
- Use `goimports` to manage import grouping: stdlib → external → internal.
- Indentation is **tabs**, not spaces (enforced by `go fmt`).
- Max line length: **120 characters** (soft limit, enforced by linter).
- Keep functions short and focused. If a function exceeds ~40 lines, consider splitting it.

---

### 3. Naming Conventions

- **Packages**: short, lowercase, single word (e.g., `handler`, `service`, `repository`). No underscores.
- **Interfaces**: named by behavior, ending in `-er` when appropriate (e.g., `BookRepository`, `AuthService`).
- **Acronyms**: fully uppercase (e.g., `HTTPServer`, `userID`, `parseJSON`).
- **Unexported identifiers**: use `camelCase`. Exported identifiers: use `PascalCase`.
- **Variables**: prefer descriptive names over short ones except for well-known idioms (`err`, `ctx`, `i`, `ok`).
- **Constants**: use `PascalCase` for exported, `camelCase` for unexported. Group related constants in `const` blocks.

---

### 4. Interfaces & Dependency Injection

- **Define interfaces in the consumer package** (e.g., `service` defines the `BookRepository` interface it needs).
- Keep interfaces **small and focused** — one responsibility per interface.
- Use **constructor-based dependency injection** everywhere.
- Do not use global state or singletons. Pass all dependencies explicitly.

```go
// service/book.go — interface defined where it's consumed
type BookRepository interface {
    FindByID(ctx context.Context, id string) (*domain.Book, error)
    Save(ctx context.Context, book *domain.Book) error
}

type BookService struct {
    repo BookRepository
}

func NewBookService(repo BookRepository) *BookService {
    return &BookService{repo: repo}
}
```

---

### 5. Project Layer Responsibilities

| Layer        | Responsibility                                                    |
| :----------- | :---------------------------------------------------------------- |
| `handler`    | HTTP binding, request validation, response serialization          |
| `service`    | Business logic, orchestration, domain rules                       |
| `repository` | Data access only — no business logic                              |
| `domain`     | Pure structs, sentinel errors, value objects — no external deps   |
| `config`     | Load and validate environment/config — fail fast on missing vars  |

- Handlers **must not** call repositories directly. Always go through the service layer.
- Services **must not** import `gin` or any HTTP-related packages.
- Repositories **must not** contain business logic.

---

### 6. Context Usage

- Always accept `context.Context` as the **first parameter** in service and repository functions.
- Propagate context from the Gin handler through every layer.
- Use context for cancellation, deadlines, and request-scoped values (e.g., request ID).
- Never store a context inside a struct — pass it at call time.

```go
// Good
func (r *MongoBookRepository) FindByID(ctx context.Context, id string) (*domain.Book, error)

// Bad
type MongoBookRepository struct {
    ctx context.Context // never do this
}
```

---

### 7. Concurrency

- Avoid goroutine leaks — every goroutine must have a clear exit path.
- Use `sync.WaitGroup` or `errgroup` when fanning out work.
- Run tests and the server with the race detector during development: `go run -race` / `go test -race ./...`.
- Prefer channels for communication; prefer mutexes for state protection.

---

### 8. Testing

- Write **table-driven tests** for all business logic in `service` and `repository` layers.
- Place test files next to the code they test (`foo_test.go`).
- Use interfaces to mock dependencies — do not hit real databases in unit tests.
- Aim for **≥ 95% coverage** on the `service` package.
- Integration tests (requiring MongoDB) go in a separate `_test` package and are gated by a build tag.

```go
func TestFindBook(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        want    *domain.Book
        wantErr error
    }{
        {"found", "123", &domain.Book{ID: "123"}, nil},
        {"not found", "999", nil, domain.ErrNotFound},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { /* ... */ })
    }
}
```

---

### 9. Logging

- Use Go's standard `log/slog` with **structured, JSON output**.
- Do **not** use `fmt.Println` or `log.Printf` for application logs.
- Always include context-bound fields: `request_id`, `user_id`, `method`, `path`.
- Log at the **handler layer** for request/response; log errors at the **service layer** only when they cannot bubble up further.
- Use `slog.Error` for unexpected errors, `slog.Warn` for recoverable issues, `slog.Info` for lifecycle events.

---

### 10. Configuration

- Load all configuration from **environment variables**. No hardcoded values.
- Validate required variables at startup; **fail fast** with a clear error message if any are missing.
- Use a dedicated `Config` struct in the `config` package.

```go
type Config struct {
    MongoURI string
    Port     string
    Env      string
}
```

---

### 11. API & Validation

- Use `go-playground/validator` tags on request structs. Validate in the handler before calling the service.
- Return structured JSON error responses — never expose internal error details to clients.
- Use appropriate HTTP status codes (`400` for bad input, `404` for not found, `409` for conflicts, `500` for internal errors).
- Document all endpoints with OpenAPI/Swagger specs in the `api/` directory.

---

### 12. Tools & Linting

| Make Target      | Tool             | Purpose                                 |
| :--------------- | :--------------- | :-------------------------------------- |
| `make fmt`       | `go fmt` + `goimports` | Code formatting and import ordering |
| `make lint`      | `golangci-lint`  | Static analysis                         |
| `make test-race` | `go test -race`  | Race condition detection                |
| `make tidy`      | `go mod tidy`    | Dependency hygiene                      |

