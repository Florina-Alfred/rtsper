Purpose
- This file guides agentic coding agents working in this repository.
- It documents build, lint, and test commands and enforces code style and workflow conventions.

Repository Snapshot
- Primary module file: `go.mod:1`
- This project is a Go module. The service accepts RTSP publishers on :9191 and RTSP subscribers on :9192.

Quick Commands
- Build module dependencies: `go mod download`
- Build (compile) project: `go build ./...`
- Run all tests: `go test ./...`
- Run tests with verbose output: `go test -v ./...`
- Run a single package tests (from repo root): `go test ./path/to/pkg -v`
- Run a single test by name (package-level): `go test ./path/to/pkg -run '^TestName$' -v`
- Format code: `gofmt -w .`
- Fix imports: `gofmt -w . && goimports -w .` (install `goimports` if not present)
- Vet checks: `go vet ./...`
- Static analysis: `staticcheck ./...` (recommended)

Running a Single Test — Examples
- From repository root, package `pkg/topic` run `TestDispatcherBackpressure`:
  - `go test ./pkg/topic -run '^TestDispatcherBackpressure$' -v`

CI / Automation
- Agents should prefer the repository's CI definitions if present (e.g., `.github/workflows/*`) for authoritative commands.

Environments & Prerequisites
- Go toolchain: `go 1.22` or compatible (see `go.mod:1`).
- Install helper tools when needed:
  - `go install golang.org/x/tools/cmd/goimports@latest`
  - `go install honnef.co/go/tools/cmd/staticcheck@latest`

Code Style Guidelines — High Level
- Language: Go.
- Keep code simple and idiomatic: prefer readability and clarity.
- Follow the official Go style: `gofmt`/`goimports` enforced.
- Avoid long functions; aim for short, focused functions.

Imports
- Order imports as: standard library, blank line, third-party, blank line, internal packages.
- Use `goimports` to automatically group and remove unused imports.

Formatting
- Run `gofmt -w .` before committing.
- Use `goimports -w .` to fix imports.

Types and Interfaces
- Exported types and functions: CamelCase starting with capital letter.
- Unexported: mixedCase starting with lowercase letter.
- Keep interfaces small and focused.

Naming Conventions
- Use common Go initialisms in ALL CAPS: `HTTPClient`, `ID`, `URL`, `JSON`.
- Package names short and singular.
- Test functions: `TestFoo`, `TestFoo_BarCase` or subtests with `t.Run`.

Error Handling
- Check errors immediately; do not ignore them unless justified.
- Wrap errors with `%w` to preserve original error.
- Avoid panic except for unrecoverable programmer errors or in tests.

Context
- Long-running or cancellable public functions should accept `context.Context` as the first parameter.
- Propagate contexts downwards and never store contexts in structs.

Logging
- Prefer structured logging if present. Otherwise use `log`.
- Log at system boundaries, not deep inside libraries.

Concurrency
- Prefer channels and goroutines with clear ownership semantics.
- Protect shared mutable state with mutexes; avoid global mutable state.

Tests
- Use table-driven tests for variations.
- Tests must be independent and deterministic.
- Mock external dependencies via interfaces when needed.

Error Messages & Strings
- Error messages should be lower-case and not end with punctuation.
- Avoid leaking implementation details in errors.

Security
- Do not commit secrets or API keys. Use environment variables for secrets.

Testing & Coverage
- Run `go test ./... -cover` to get coverage summary.

Refactoring & PRs
- Small, focused PRs are preferred.
- Run `gofmt`, `go vet`, and linter before creating PRs.

Tooling & Editors
- Agents should rely on `gofmt`, `goimports`, `go vet`, `staticcheck`, and `golangci-lint` where helpful.

Cursor / Copilot Rules
- No `.cursor/rules/` or `.cursorrules` directory found in repository root.
- No `.github/copilot-instructions.md` found.

Agent Behavior and Safety
- Always read `AGENTS.md` before making changes.
- Prefer non-destructive actions; ask for user approval before creating many files, running destructive commands, or committing/pushing changes.

What to do when files are missing / unknown
- Propose minimal config and ask for approval before adding it.

On Writing Files
- Only create new files if change is approved by repository owner.
- When creating/modifying files, include concise comments explaining intent.

Local Repro Steps for Developers
- Ensure Go is installed: `go version` (target `go1.22+`).
- Install helper tools:
  - `go install golang.org/x/tools/cmd/goimports@latest`
  - `go install honnef.co/go/tools/cmd/staticcheck@latest`
- Run quick verification:
  - `gofmt -w .`
  - `goimports -w .`
  - `go vet ./...`
  - `staticcheck ./...` (optional)
  - `go test ./... -v`

Contact & Follow-up
- If agents detect missing conventions or want to add linter/CI configs, propose changes with a short rationale and ask for approval.

