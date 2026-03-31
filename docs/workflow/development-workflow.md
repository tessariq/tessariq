# Development Workflow

Contributor and coding-agent workflow for tracked work in Tessariq.

## Source Of Truth

- `Taskfile.yml`
- `.github/workflows/ci.yml`
- `docs/workflow/`
- `planning/STATE.md`
- `planning/tasks/`

## Build And Validation

- Build: `go build ./cmd/tessariq`
- Product tests: `go test ./...`
- Integration tests: `go test -tags=integration ./...`
- End-to-end tests: `go test -tags=e2e ./...`
- Mutation tests: `gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70`
- Workflow validation: `go run ./cmd/tessariq-workflow validate-state`
- Workflow validation bundle: `task workflow:check`
- Skill parity: `go run ./cmd/tessariq-workflow check-skills`
- Spec verification: `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json` (active milestone spec only)

## TDD Default

For any code change:

1. Write the smallest failing test that captures the behavior.
2. Make that test pass with the minimal implementation.
3. Refactor while keeping the test suite green.

If a unit test cannot express the behavior safely, move one level up the testing pyramid.

## Testing Pyramid

- Unit tests:
  the default layer and the bulk of behavioral coverage
- Integration tests:
  validate subsystem boundaries and real collaborators
- End-to-end tests:
  prove only the most important user-visible CLI workflows

Rules:

- Unit tests must use in-memory data only and must not touch real files, temp files, Docker, or network.
- Integration tests may use temporary files and workspaces, but service and process collaborators must come from Testcontainers for Go.
- End-to-end tests may use temporary workspaces and Testcontainers for Go, but not custom local servers.
- Integration and e2e tests must not call live external services.

Testcontainers standard:

- use `github.com/testcontainers/testcontainers-go`
- use official wait strategies
- keep container-backed suites on Linux CI runners with Docker available
- all reusable container helpers live in `internal/testutil/containers/`
- available helpers: `StartGitRepo` (git), `StartHTTPBin` (HTTP), `StartAgentEnv` (agent process), `StartRunEnv` (full CLI e2e with tmux+git+fake claude)
- new process or service collaborators must get a `Start*` helper — do not create ad-hoc local fakes or depend on host-installed tools
- e2e tests must use `StartRunEnv` so they are self-contained and CI-portable
- build CLI binaries with `CGO_ENABLED=0` when targeting Alpine containers

## Mutation Testing

- Gremlins is part of the normal verification ladder.
- CI enforces `gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70`.
- Use mutation testing for non-trivial logic changes and when logic-confidence evidence would otherwise be weak.

## Manual Testing

After automated test tiers pass, run the `autonomous-manual-test` skill to exercise the built CLI against the task's acceptance criteria:

1. The agent reads the task's acceptance criteria and generates a test plan.
2. Each test step runs in the appropriate mode:
   - **Sandbox mode**: standalone Go programs in `/tmp/tessariq-manual-test-<task-id>/` for API-level tests.
   - **Container mode**: `_manual_test.go` files with `//go:build manual_test` tag for tests needing tmux, fake adapter binaries, or full CLI lifecycle.
3. Failures are classified by severity (critical, major, minor) and resolved inline when possible.
4. A structured report records all outcomes.
5. Artifacts are written to `planning/artifacts/manual-test/<task-id>/<timestamp>/`.

Container mode manual tests:
- Place `_manual_test.go` files in the package closest to the code under test.
- Name test functions `TestManual_<descriptive name>`.
- Use Testcontainers helpers from `internal/testutil/containers/`.
- Run via `go test -tags=manual_test ./<package>/ -run TestManual_<Name> -v -count=1`.
- Build CLI binaries with `CGO_ENABLED=0` for Alpine containers.
- Never substitute automated e2e test results for manual test evidence.

Cleanup (critical):
- Manual test code is **ephemeral** — only artifacts (plan.md, report.md) are persisted.
- After the report is written, delete all `_manual_test.go` files and `cmd/manual-test-*/` directories.
- `.gitignore` blocks these patterns as a safety net, but agents must still clean up explicitly.
- Never commit manual test code.

Manual testing is required before running verification and before finishing a task as `done`.

## Tracked-Work Commands

- `go run ./cmd/tessariq-workflow validate-state`
- `go run ./cmd/tessariq-workflow next --json`
- `go run ./cmd/tessariq-workflow start --mode user_request --agent-id <agent> --model <model> <task-id>`
- `go run ./cmd/tessariq-workflow finish --status done --note "<evidence>" <task-id>`
- `go run ./cmd/tessariq-workflow refresh-state`
- `go run ./cmd/tessariq-workflow verify --profile task|implemented|spec --disposition report|hybrid --json`
- `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`

Notes:

- `verify --profile task` now includes a medium-severity reminder finding when user-visible code changes are detected without updating `CHANGELOG.md`; workflow-tooling-only changes (`cmd/tessariq-workflow/`, `internal/workflow/`) are excluded.

## Commit Policy For Tracked Tasks

- Use exactly one commit per tracked implementation task.
- The commit must use a conventional commit message.
- Include implementation changes, tests, and required workflow/planning artifact updates in that same commit.
- Do not create a separate follow-up commit only for verification/planning/task metadata updates.

## Change-Type Matrix

- Docs-only changes:
  no code validation unless runnable examples changed
- Small code changes:
  TDD plus targeted unit tests
- Cross-package logic changes:
  unit tests, integration tests if boundaries changed, and mutation tests
- CLI workflow changes:
  unit tests plus targeted e2e coverage
- Tracked-work system changes:
  workflow validation, skill parity check, and spec verification
- Spec or planning edits:
  `validate-state` and `verify --profile spec` are hard failure gates and must pass before review
- All code changes:
  manual testing against task acceptance criteria before verification
