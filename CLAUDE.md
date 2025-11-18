# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KubeMin-Cli is a developer-friendly Kubernetes application platform written in Go that provides a high-level abstraction layer for deploying and orchestrating services on Kubernetes. It implements a lightweight workflow engine as a custom Kubernetes controller, inspired by the Open Application Model (OAM).

## Common Development Commands

### Build and Run
- `go run ./cmd/main.go` - Run the API server locally using default settings
- `go build -o kubemin-cli cmd/main.go` - Build binary for current OS
- `make build-linux` - Cross-compile for Linux amd64
- `make build-darwin` - Cross-compile for macOS amd64
- `make build-windows` - Cross-compile for Windows amd64
- `make docker-build` - Build multi-arch Docker images (linux/amd64, linux/arm64)

### Testing
- `go test ./... -race -cover` - Run all tests with race detection and coverage (required before committing)
- `go test ./pkg/apiserver/workflow/... -v` - Run specific package tests with verbose output
- `go test -run TestName ./path/to/package` - Run a specific test

### Code Quality
- `go fmt ./...` - Format all Go code
- `go vet ./...` - Run static analysis
- `go mod tidy` - Clean up module dependencies

## Architecture Overview

### Clean Architecture Structure
```
pkg/apiserver/
├── domain/           # Business logic and domain models
│   ├── model/        # Domain entities (Application, Component, Workflow)
│   ├── service/      # Business logic services
│   └── repository/   # Data access interfaces
├── infrastructure/   # External integrations
│   ├── persistence/  # Database layer (GORM + MySQL)
│   ├── messaging/    # Queue implementations (Redis Streams, local)
│   ├── kubernetes/   # K8s client and utilities
│   └── tracing/      # OpenTelemetry integration
├── interfaces/api/   # REST API layer
│   ├── handlers/     # HTTP request handlers
│   └── middleware/   # Gin middleware (tracing, auth)
├── workflow/         # Workflow execution engine
│   ├── dispatcher/   # Job distribution logic
│   ├── worker/       # Job execution workers
│   └── traits/       # Component trait processors
└── utils/            # Shared utilities
```

### Key Architectural Patterns

1. **Dependency Injection**: Custom IoC container (`pkg/apiserver/infrastructure/persistence/container.go`) manages service lifecycle
2. **Queue Abstraction**: Unified interface supporting Redis Streams (distributed) and local channel-based queues
3. **Trait System**: Extensible component augmentation through trait processors (storage, networking, RBAC, etc.)
4. **Leader Election**: Kubernetes Lease-based leader election for distributed mode
5. **Distributed Workflow**: Dispatcher/Worker pattern with Redis Streams for job distribution

### Core Components

- **Application**: Top-level entity containing components and workflows
- **Component**: Runnable unit (container, job) with traits applied
- **Workflow**: Orchestrates component deployment with dependencies
- **Trait**: Augments components with operational capabilities

## Development Guidelines

### Testing Requirements
- All tests must pass with `-race` flag enabled
- Use table-driven tests with testify assertions
- Mock external dependencies (Redis, K8s) when testing in isolation
- Test files must be co-located with source code as `*_test.go`

### Code Conventions
- Use `k8s.io/klog/v2` for logging (not fmt or log)
- Pass `context.Context` to all goroutines
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Follow existing package structure and naming conventions

### Key Dependencies
- **Web Framework**: Gin for REST API
- **Kubernetes**: controller-runtime for K8s integration
- **Database**: GORM with MySQL
- **Queue**: Redis Streams with local fallback
- **Observability**: OpenTelemetry with Jaeger

### Configuration
- Environment variables take precedence over config files
- Key env vars: `REDIS_ADDR`, `MYSQL_DSN`, `KUBECONFIG`
- Configuration validation in `pkg/apiserver/config/`

### Common Development Tasks

When modifying workflow execution:
1. Check `pkg/apiserver/workflow/dispatcher/` for job distribution logic
2. Review `pkg/apiserver/workflow/worker/` for execution patterns
3. Update both local and Redis queue implementations if changing interfaces

When adding new traits:
1. Implement trait processor in `pkg/apiserver/workflow/traits/`
2. Register processor in `pkg/apiserver/workflow/traits/processor.go`
3. Add comprehensive tests following existing patterns
4. Update API handlers if trait requires new parameters

When working with Kubernetes resources:
1. Use utilities in `pkg/apiserver/utils/kube/` for common operations
2. Follow existing patterns for RBAC, ConfigMaps, and Secrets
3. Ensure proper cleanup in failure scenarios

### Commit Format
Follow `type: description` format:
- `feat:` - New features
- `fix:` - Bug fixes
- `refactor:` - Code refactoring
- `test:` - Test additions/changes
- `docs:` - Documentation updates