# Contributing to goro-pg

Thank you for your interest in contributing! This document covers how to set up, develop, and submit changes.

## Development Setup

1. **Prerequisites:** Go 1.23+, Docker, a C compiler (for `pg_query_go`)

2. **Start a local PostgreSQL instance:**

   ```bash
   docker compose up -d
   ```

3. **Run tests:**

   ```bash
   CGO_ENABLED=1 go test ./... -race
   ```

4. **Build the binary:**

   ```bash
   CGO_ENABLED=1 go build -o goro-pg ./cmd/main.go
   ```

   Or via Docker:

   ```bash
   docker build -t goro-pg .
   ```

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/) and [release-please](https://github.com/googleapis/release-please) for automated releases. Commit messages must follow this format:

```
<type>: <description>

[optional body]
```

Common types:
- `feat:` — new feature (triggers minor version bump)
- `fix:` — bug fix (triggers patch version bump)
- `test:` — adding or updating tests
- `docs:` — documentation changes
- `chore:` — maintenance tasks
- `refactor:` — code restructuring without behavior change

Breaking changes: add `!` after the type (e.g. `feat!: remove legacy endpoint`) or include `BREAKING CHANGE:` in the footer.

## Pull Request Workflow

1. Branch from `master`
2. Make your changes
3. Ensure all tests pass: `go test ./... -race`
4. Ensure code is formatted: `gofmt -w .`
5. Ensure vet passes: `go vet ./...`
6. Open a PR against `master`
7. CI must pass before merge

## Contributing Adversarial Tests

The read-only guard is a critical security component. We welcome adversarial test contributions that attempt to bypass it. See the [Contributing Adversarial Tests](README.md#contributing-adversarial-tests) section in the README for details on how to add test cases to `internal/guard/adversarial_test.go`.

## Code Style

- Format with `gofmt`
- Follow standard Go conventions
- Keep changes focused — one concern per PR
