# Repository Guidelines

## Project Structure & Module Organization
Source entrypoints live in `cmd/` (primary CLI in `cmd/main.go`). Core services run under `pkg/apiserver/` across domain, interfaces, infrastructure, utilities, and workflow layers. Shared libraries belong in `pkg/`. Configuration, manifests, docs, examples, and scripts reside in `configs/`, `deploy/`, `docs/` or `doc/`, `examples/`, and `scripts/`. Tests sit beside their targets as `*_test.go`.

## Build, Test, and Development Commands
- `go run ./cmd/main.go` – run the CLI/apiserver locally using default settings.
- `go build -o kubemin-cli cmd/main.go` – compile the CLI binary for your OS.
- `make build-apiserver` – cross-build the apiserver for Linux/amd64 deployments.
- `go test ./... -race -cover` – execute all unit tests with race detection and coverage.
- `scripts/start-distributed.sh` – launch the distributed demo; ensure Redis env vars are set.

## Coding Style & Naming Conventions
- Target Go 1.24; format with `go fmt ./...` and lint with `go vet ./...`.
- Keep package paths lowercase and add new modules under cohesive `pkg/...` boundaries.
- Use `k8s.io/klog/v2` for logging; wrap errors as `fmt.Errorf("create pvc: %w", err)`.
- Pass `context.Context` into goroutines, preferring `errgroup` or `sync.WaitGroup` for coordination.

## Testing Guidelines
- Co-locate tests as `*_test.go`; favor table-driven cases and testify assertions.
- Cover edge and error paths, adding minimal fixtures near the package when needed.
- Run `go test ./... -race -cover` before committing and note coverage deltas in PRs.
- Mock Redis-aware components when isolation is required; use the distributed script for integration checks.

## Commit & Pull Request Guidelines
- Follow `type: short description` commits, e.g., `fix: reconcile worker locks`; keep each change focused.
- PRs must state what/why, call out risks, attach test evidence, and link issues. Include CLI or curl examples for behavior shifts.
- Update docs under `docs/` or `doc/` for API or workflow changes; keep manifests in `deploy/` with explicit images.

## Security & Configuration Tips
- Avoid hardcoded secrets; rely on config files or environment variables like `REDIS_ADDR`.
- Prefer `Cache.CacheHost` over CLI flags for distributed queue targets.
- Set resource requests/limits for new manifests and mask sensitive data in logs.
