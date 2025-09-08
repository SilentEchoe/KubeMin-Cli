# Repository Guidelines

## Project Structure & Module Organization
- Source: `cmd/` (entrypoints), `pkg/apiserver/` (domain, interfaces, infrastructure, utils, workflow), reusable libs in `pkg/`.
- Configuration & assets: `configs/`, `deploy/` (manifests), `docs/` and `doc/` (design notes), `examples/` (usage), `scripts/` (ops/dev helpers).
- Tests live next to code as `*_test.go` (e.g., `pkg/apiserver/workflow/...`).

## Build, Test, and Development Commands
- Run locally: `go run ./cmd/main.go`
- Build CLI: `go build -o kubemin-cli cmd/main.go`
- Make target: `make build-apiserver` (builds `pkg/apiserver/server.go` for Linux/amd64)
- Tests: `go test ./... -race -cover`
- Distributed demo: `scripts/start-distributed.sh` (uses Redis; see env below)

## Coding Style & Naming Conventions
- Language: Go 1.24. Format with `go fmt ./...`; validate with `go vet ./...`.
- Logging: use `k8s.io/klog/v2` (no `fmt.Println`).
- Errors: wrap with context, e.g., `fmt.Errorf("create pvc: %w", err)`.
- Concurrency: pass `context.Context` to goroutines; prefer `errgroup`/`sync.WaitGroup`.
- Packages/paths: lower-case package names; place new modules under `pkg/...` with cohesive boundaries.

## Testing Guidelines
- Frameworks: standard `testing` + `testify` assertions.
- Location: keep tests beside implementation; name `*_test.go`.
- Style: prefer table-driven tests; cover error paths and edge cases.
- Run: `go test ./... -race -cover` before pushing; add minimal fixtures under the package, not global temp dirs.

## Commit & Pull Request Guidelines
- Commit style: `type: short description` (e.g., `feat: enable tracing`, `fix: configmap input`). Optional scope allowed.
- Keep commits focused and descriptive; include rationale in body when non-obvious.
- PRs must include: what/why, notable risks, test evidence (output or `go test` summary), and linked issues.
- For API/behavior changes, include examples (flags, curl, or CLI usage) and update docs under `docs/` if needed.

## Security & Configuration Tips
- Do not hardcode secrets; use environment or config and mask in logs.
- Common flags: `--bind-addr`, `--max-workers`, `--id`, `--lock-name`.
- Distributed queue config: prefer `Cache.CacheHost` (e.g., `redis:6379`) in config over CLI flags; env `REDIS_ADDR` is a fallback.
- Distributed script env: `REDIS_HOST`, `REDIS_PORT`, `MAX_WORKERS`, `NODE_ID`, `BIND_ADDR`.
- Manifests belong in `deploy/`; avoid `:latest` images and always set requests/limits.
