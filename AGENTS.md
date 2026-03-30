# AGENTS.md
Guidance for coding agents working in the Tessariq repository.

## Scope and intent
- This is a Go CLI repository: `github.com/tessariq/tessariq`.
- Main executable: `./cmd/tessariq`.
- Internal packages: `./internal/...`.
- Product specs: `./specs/...`.
- Keep changes small and behavior-preserving unless explicitly requested.
- Runtime state lives under `.tessariq/` and is never committed.

## Source-of-truth files
- Product specifications: `specs/tessariq-v0.1.0.md` and `specs/tessariq-v0.2.0.md`.
- Spec reading order and versioning policy: `specs/README.md`.
- Commands and local workflows: `Taskfile.yml`.
- CI checks and required validations: `.github/workflows/ci.yml`.
- Release pipeline: `.goreleaser.yml`.
- Product overview: `README.md`.
- Tracked-work workflow and testing policy: `docs/workflow/`.
- Mirrored agent skills: `.agents/skills/` and `.claude/skills/`.

## Toolchain and environment
- Go version: `1.26` (`go.mod`).
- `task` is optional convenience; Go commands are the source of truth.
- Prefer commands that mirror CI.
- Docker is a runtime dependency for `tessariq run`; it is not needed for building or testing the CLI itself.

## Build, lint, and test commands
Use these defaults unless a task requires otherwise.

### Build
- Build CLI binary: `go build ./cmd/tessariq`
- Task wrapper: `task build`

### Formatting and static checks
- Check formatting (CI): `gofmt -l .`
- Apply formatting: `gofmt -w .`
- Vet: `go vet ./...`

### Unit tests
- Run all unit tests: `go test ./...`
- Task wrapper: `task test`

### Integration tests
- Run integration-tag tests: `go test -tags=integration ./...`
- Task wrapper: `task test:integration`
- Integration and e2e tests must use Testcontainers for Go for service or process collaborators; do not use custom local servers.

### End-to-end tests
- Run e2e-tag tests: `go test -tags=e2e ./...`
- Task wrapper: `task test:e2e`
- E2E tests should stay thin, cover critical CLI flows only, and use Testcontainers when runtime collaborators are needed.

### Run a single test
- Single test in one package:
  `go test ./internal/cli -run '^TestRootCommandShowsHelp$'`
- Single test by pattern:
  `go test ./internal/cli -run '^TestRootCommand'`
- Single integration test:
  `go test -tags=integration ./internal/workspace -run '^TestWorktreeCreation$'`
- Single test with verbose output:
  `go test -v ./cmd/tessariq -run '^TestHelpOutput$'`

### Helpful test variants
- Disable cache while iterating:
  `go test ./internal/cli -run '^TestName$' -count=1`
- Run one package only:
  `go test ./internal/workspace`

### CLI smoke checks used in CI
- `go run ./cmd/tessariq --help`

### Mutation tests
- Run mutation testing: `gremlins unleash`
- Task wrapper: `task test:mutate`
- With quality gate (used in CI): `gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70`
- Install gremlins: `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`

### Tracked-work workflow commands
- Validate state: `go run ./cmd/tessariq-workflow validate-state`
- Select next task: `go run ./cmd/tessariq-workflow next --json`
- Start task: `go run ./cmd/tessariq-workflow start --mode user_request --agent-id <agent> --model <model> <task-id>`
- Finish task: `go run ./cmd/tessariq-workflow finish --status done --note "<evidence>" <task-id>`
- Refresh state: `go run ./cmd/tessariq-workflow refresh-state`
- Verify spec coverage: `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json`
- Check mirrored skills: `go run ./cmd/tessariq-workflow check-skills`
- See `docs/workflow/` for the full deterministic workflow contract.

### License compliance check used in CI
- Check allowed licenses: `go-licenses check ./cmd/tessariq --allowed_licenses=Apache-2.0,MIT,BSD-3-Clause,ISC,AGPL-3.0`
- Install go-licenses: `go install github.com/google/go-licenses/v2@v2.0.1`

### Release-related commands
- Validate GoReleaser config: `goreleaser check` (or `task release:check`)
- Dry snapshot release: `goreleaser release --snapshot --clean`

## Coding style guidelines
Follow existing conventions and keep CLI UX stable.

### Formatting and structure
- Always run `gofmt` on changed Go files.
- Keep functions focused and prefer early returns for error paths.
- Avoid unnecessary abstractions; match current package boundaries.

### Imports
- Let `gofmt` handle import order.
- Keep stdlib imports separated from non-stdlib imports.
- Prefer module-local imports (`github.com/tessariq/tessariq/internal/...`) for internal reuse.

### Types and API shape
- Exported names: `PascalCase`; unexported names: `camelCase`.
- Sentinel errors should use `ErrXxx` naming (for example `ErrDirtyRepo`).
- Exported constructors should use `NewXxx`.
- Use explicit config structs (for example `run.Config`, `workspace.Options`).
- Prefer typed structs for core flows instead of `map[string]any`.

### Naming conventions
- Package names are short, lowercase, and noun-like.
- CLI command builders use `newXCmd` for unexported command constructors.
- Booleans should read clearly (`unsafeEgress`, `noTrailers`, `attachMode`).
- Error text should be lowercase and not end with punctuation.

### Error handling
- Return errors instead of panicking.
- Wrap underlying errors with context using `%w`.
  Example: `fmt.Errorf("create worktree for run %s: %w", runID, err)`
- Use `errors.Is` and `errors.As` for known error branches.
- Keep errors actionable for CLI users.

### Context and I/O
- Pass `context.Context` as the first parameter when operations are cancelable.
- Use `exec.CommandContext` and context-bound HTTP requests.
- Prefer `filepath` (not `path`) for filesystem operations.
- Normalize user-provided paths with `filepath.Clean` when appropriate.

### Logging and user output
- Use structured logging with `zap` for diagnostics.
- Write user-facing command output via command writers (`cmd.OutOrStdout()` / stderr).
- Log Docker and git operations at debug level; surface failures at error level.
- Evidence-path and run-id output should go to stdout for scripting.

### Testing conventions
- Use standard `testing` + `github.com/stretchr/testify/require`.
- Use `t.Parallel()` for unit tests when safe.
- Mark integration tests with `//go:build integration`.
- Mark end-to-end tests with `//go:build e2e`.
- Follow TDD for code changes: write the smallest failing test first, then make it pass, then refactor.
- Follow the testing pyramid: default to unit tests, add integration tests for subsystem boundaries, and keep e2e tests sparse and high-signal.
- Unit tests must not touch real filesystem paths, temp files, Docker, or network.
- Integration and e2e tests may use `t.TempDir()` for local fixtures and workspaces, but must use Testcontainers for Go for real process or service collaborators.
- Integration and e2e tests must not use custom HTTP/TCP servers or live external network services.
- Keep tests deterministic; avoid live Docker or network unless integration-scoped and containerized.
- Test evidence artifacts by verifying file existence, structure, and required fields.

### Testcontainers patterns
All reusable container helpers live in `internal/testutil/containers/`. Available helpers:
- `StartGitRepo` — Alpine container with git for repository operation tests.
- `StartHTTPBin` — kennethreitz/httpbin container for HTTP service tests.
- `StartAdapterEnv` — Alpine container with a configurable fake adapter binary for adapter process lifecycle tests.
- `StartRunEnv` — Alpine container with tmux, git, bash, and a fake claude binary for full CLI e2e tests.

Pattern: `Start*` → `t.Cleanup()` handles teardown → use `Exec()` for commands → use bind-mounts for host-side artifact verification.

Rules:
- Never use local fake binaries, custom local servers, or host-installed tools as process collaborators in integration or e2e tests.
- Never use `skipIfNoTmux` or similar host-tool guards — the container must provide everything.
- When a new process or service collaborator is needed, add a new `Start*` helper to `internal/testutil/containers/` rather than creating ad-hoc local fakes.
- Wait strategies: use `wait.ForExec` for process-based containers, `wait.ForHTTP` for service containers.
- Build CLI binaries with `CGO_ENABLED=0` when they run inside Alpine containers (glibc vs musl).
- Run containers as the current user (`user.Current()`) when bind-mounting host dirs, or fix ownership in cleanup to avoid `t.TempDir()` permission errors.

### Integration test checklist
- Build tag: `//go:build integration`.
- Process and service collaborators must come from a `containers.Start*` helper.
- Local fixtures via `t.TempDir()` are fine for files the test itself creates.
- No host tool dependencies (tmux, docker CLI, etc.) — these must be inside the container.
- Use `env.Exec()` to run commands inside the container and assert on exit codes and output.

### E2e test checklist
- Build tag: `//go:build e2e`.
- Use `StartRunEnv` (or similar) to get a container with all runtime dependencies.
- Build the CLI binary on the host with `CGO_ENABLED=0`, copy into bind-mount.
- Run CLI commands inside the container via `env.Exec(ctx, []string{"sh", "-c", "cd /repo && /work/binary ..."})`.
- Read evidence artifacts from inside the container via `env.Exec(ctx, []string{"cat", path})`.
- No `skipIfNoTmux` or similar host-tool guards.

### Dependency and platform practices
- Prefer the standard library first; add dependencies only when justified.
- Keep Linux/macOS differences explicit and runtime-guarded.
- Shell out to `git` and `docker` via `exec.CommandContext`; do not embed git libraries.
- Squid proxy configuration should be generated, not templated from embedded files.

## Domain-specific guidance

### Docker and containers
- Each run creates an isolated container; use deterministic container names (`tessariq-<run_id>`).
- Prefer `docker create` + `docker start` over `docker run` for lifecycle control.
- Always clean up containers and networks on error paths.

### Git worktrees
- Worktrees live under `~/.tessariq/worktrees/<repo_id>/<run_id>`.
- Use `git worktree add --detach` to avoid branch-name conflicts.
- Always `git worktree remove` during cleanup.

### Networking and Squid proxy
- Proxy mode creates a per-run Docker network and Squid container.
- Allowlists are enforced at `host:port` granularity; default port is 443.
- Record compiled egress rules in `egress.compiled.yaml` for auditability.

### Evidence artifacts
- Every run must emit the required evidence files per the spec.
- Evidence paths are deterministic: `<repo>/.tessariq/runs/<run_id>/`.
- Status, manifest, and workspace JSON must be valid even when a run fails.

## Change checklist for agents
- Run `gofmt -w` on edited Go files.
- Run `go vet ./...` for non-trivial changes.
- Keep implementation in a TDD loop for code changes.
- Run targeted tests for touched packages.
- Run full `go test ./...` before handing off broad changes.
- If integration paths changed, run `go test -tags=integration ./...`.
- If e2e paths changed, run `go test -tags=e2e ./...`.
- If non-trivial logic changed, run `gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70`.
- If tracked-work tooling or skills changed, run `go run ./cmd/tessariq-workflow validate-state`, `go run ./cmd/tessariq-workflow check-skills`, and `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json`.
- Update `README.md` when CLI flags/commands/behavior change.
- Update `CHANGELOG.md` for user-visible behavior changes; keep entries user-facing and skip internal-only maintenance noise.
- Verify evidence file contracts are maintained when changing run or promote logic.
- Run manual testing against the task's acceptance criteria before verification; artifacts must exist under `planning/artifacts/manual-test/<task-id>/` before finishing as `done`.
- After manual testing, delete all manual test code (`_manual_test.go` files and `cmd/manual-test-*/` directories); only artifacts (plan.md, report.md) are committed.
- Update specs in `specs/` only when explicitly requested; specs are normative.

## Agent do/don'ts (PR + commits)
- Do keep branches and PRs focused on one logical change.
- Do include a clear PR summary with why, what changed, and test evidence.
- Do keep commits small and descriptive, using imperative commit subjects.
- Do use conventional commit messages.
- Do mention user-visible CLI changes in the PR body.
- Don't mix unrelated refactors or formatting-only churn into feature/fix PRs.
- Don't rewrite the shared branch history unless explicitly requested.
- Don't bypass CI-equivalent checks before asking for review.
- Don't modify spec files unless the change is explicitly about spec updates.

## Notes on repository behavior
- Default CLI flow is `run -> attach if needed -> promote`.
- Tessariq refuses to start a run on a dirty repository.
- Evidence artifacts are written even when a run fails.
- Runtime state under `.tessariq/` is never committed.
