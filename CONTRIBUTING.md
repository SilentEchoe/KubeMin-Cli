# Contributing

Thanks for your interest in KubeMin-Cli! This repo follows a lightweight contribution flow.

## Development

- Go 1.24 is the target version.
- Format and lint:
  - `go fmt ./...`
  - `go vet ./...`
- Run tests:
  - `go test ./... -race -cover`

## Local Run

- `go run ./cmd/main.go`

## Commit Style

Use short, focused commits in the form:

- `type: short description`

Examples:

- `fix: reconcile worker locks`
- `feat: add queue metrics`

## Pull Requests

Include:

- What/why summary and any risks
- Test evidence (commands + results)
- Example CLI or curl usage for behavior changes

