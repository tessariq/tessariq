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
- Skill parity: `go run ./cmd/tessariq-workflow check-skills`
- Spec verification: `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json`

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

## Mutation Testing

- Gremlins is part of the normal verification ladder.
- CI enforces `gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70`.
- Use mutation testing for non-trivial logic changes and when logic-confidence evidence would otherwise be weak.

## Tracked-Work Commands

- `go run ./cmd/tessariq-workflow validate-state`
- `go run ./cmd/tessariq-workflow next --json`
- `go run ./cmd/tessariq-workflow start --mode user_request --agent-id <agent> --model <model> <task-id>`
- `go run ./cmd/tessariq-workflow finish --status done --note "<evidence>" <task-id>`
- `go run ./cmd/tessariq-workflow refresh-state`
- `go run ./cmd/tessariq-workflow verify --profile task|implemented|spec --disposition report|hybrid --json`
- `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`

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
