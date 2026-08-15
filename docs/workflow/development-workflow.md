# Development Workflow

Contributor and coding-agent workflow for tracked work in Tessariq.

## Build And Validation

The developer toolchain is pinned in `mise.toml`; `mise install` (or `mise run
setup`) provisions it, and CI runs the same `task` targets after provisioning
through the repository's shared setup action. Direct `go` commands remain canonical.

- Build: `task build` (`go build ./cmd/tessariq`)
- Product tests: `task test` (`go test ./...`)
- Integration tests: `task test:integration` (`go test -tags=integration ./...`)
- End-to-end tests: `task test:e2e` (`go test -tags=e2e ./...`)
- Differential mutation tests for local changes: `task test:mutate` (compares with `main`; override with `BASE=<ref>`)
- Full mutation gate (weekly CI and manual dispatch): `task test:mutate:gate`
- Workflow validation: `task workflow:validate` (`taskrail validate`)
- Skill provenance and parity: `task workflow:check-skills`
- Spec verification: `task workflow:verify:spec` (`taskrail coverage --json`, active milestone spec only)

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

- `task test:mutate` asks Gremlins to mutate only lines changed since `main`, making deliberate local checks practical. Set `BASE=<ref>` when the comparison base differs.
- The full baseline runs **weekly** against `main` (`.github/workflows/mutation.yml`), plus on manual dispatch. It is not part of the pull-request path because it re-runs the unit suite once per covered mutant.
- Weekly CI enforces 70% efficacy, uploads `mutation-results.json`, writes a metric summary, and maintains one `github_actions` issue until a full run recovers. Manually dispatch the workflow after a fix to close the issue immediately.
- Gremlins v0.6.0 is pinned in the Taskfile and installed on demand through `go run`. Mutation tasks clear Go's test-result cache before Gremlins times the baseline and limit execution to two workers, preventing cached timings or high-core hosts from producing false timeouts. Routine `mise install` and ordinary CI jobs do not provision it.

## Manual Testing

After automated test tiers pass, run the `autonomous-manual-test` skill to exercise the built CLI against the task's acceptance criteria:

1. The agent reads the task's acceptance criteria and generates a test plan.
2. Each test step runs in the appropriate mode:
   - **Sandbox mode**: standalone Go programs in `/tmp/tessariq-manual-test-<task-id>/` for API-level tests.
   - **Container mode**: `_manual_test.go` files with `//go:build manual_test` tag for tests needing tmux, fake adapter binaries, or full CLI lifecycle.
3. Failures are classified by severity (critical, major, minor) and resolved inline when possible.
4. A structured report records all outcomes.
5. Local-only artifacts are written to `planning/artifacts/manual-test/<task-id>/<timestamp>/`.

Container mode manual tests:
- Place `_manual_test.go` files in the package closest to the code under test.
- Name test functions `TestManual_<descriptive name>`.
- Use Testcontainers helpers from `internal/testutil/containers/`.
- Run via `go test -tags=manual_test ./<package>/ -run TestManual_<Name> -v -count=1`.
- Build CLI binaries with `CGO_ENABLED=0` for Alpine containers.
- Never substitute automated e2e test results for manual test evidence.

### Artifact Templates

The skill writes both artifacts under `planning/artifacts/manual-test/<task-id>/<timestamp>/`. These
templates are the repository contract; the skill itself is vendored and repo-agnostic, so the exact
shape lives here.

`plan.md` — one entry per test step, numbered `MT-001`, `MT-002`, …:

```
# Manual Test Plan

- Task: <task-id>
- Generated: <ISO-8601 timestamp>

## MT-001: <description derived from acceptance criterion>

- Severity: critical | major | minor
- Mode: sandbox | container
- Derived from: <quoted or paraphrased acceptance criterion>
- Setup: <preconditions or fixture creation>
- Command: `<shell command to execute>`
- Expected: <observable outcome: exit code, file existence, output content>
```

`report.md` — one entry per executed step, plus a summary:

```
# Manual Test Report

- Task: <task-id>
- Executed: <ISO-8601 timestamp>
- Verdict: pass | pass-with-fixes | fail

## Results

### MT-001: <description>

- Status: pass | fail | fixed | skipped
- Observation: <what actually happened>
- Fix: <if status is "fixed", the code change with file:line>
- Re-run: <pass | fail, only if a fix was applied>

## Summary

- Total: N | Pass: N | Fixed: N | Failed: N | Skipped: N
```

Severity drives failure handling:
- **critical** — fix the code and re-run the step. If the fix fails after one attempt, stop testing
  and write the report. A task with an unfixed critical step must not finish as `completed`.
- **major** — attempt one fix and re-run. If the fix fails, log it and continue to the next step.
- **minor** — log the observation and continue.

Verdict rules:
- **pass** — all steps passed on first run.
- **pass-with-fixes** — all steps passed, but one or more required a code fix.
- **fail** — one or more critical or major steps failed and could not be fixed.

Fixes apply to product code only; never mutate test expectations to force a pass. Re-run only the
specific failing step after a fix, not the entire plan.

Cleanup (critical):
- Manual test code is **ephemeral** and `planning/artifacts/` is gitignored.
- After the report is written, delete all `_manual_test.go` files and `cmd/manual-test-*/` directories.
- Keep only the local `plan.md` and `report.md` artifacts on disk.
- `.gitignore` blocks these patterns and `planning/artifacts/` as a safety net, but agents must still clean up explicitly.
- Never commit manual test code.
- Never commit files under `planning/artifacts/`.

Manual testing is required before running verification and before finishing a task as `completed`.

## Commit Policy For Tracked Tasks

- Use exactly one commit per tracked implementation task.
- The commit must use a conventional commit message.
- End the subject with only the short task key, for example `feat: add copy workspace mode (T-124)`. Never use the full slugged task identifier or put the key at the start. Non-tracked commits may omit a task reference.
- Include a body after the subject and blank line that explains intent, context, and non-obvious decisions rather than restating the diff. Wrap body lines at 72 characters. Generated merge, revert, fixup, and squash commits are exempt.
- Include implementation changes, tests, and required workflow/planning metadata updates in that same commit, but do not commit files under `planning/artifacts/`.
- Do not create a separate follow-up commit only for verification/planning/task metadata updates.

## Documentation And Changelog Policy

- Keep installation, quickstart, the core workflow, and the product overview in `README.md`.
- Put detailed command behavior and edge cases in `docs/commands.md`, and runtime-image behavior in `docs/runtime-images.md`.
- Record user-visible changes under `## [Unreleased]` in `CHANGELOG.md` according to `docs/workflow/changelog.md`; skip internal-only work.

## Change-Type Matrix

- Planning-lane-only changes:
  run workflow validation, skill verification, and advisory spec coverage; the
  exact routed paths are mirrored between `.github/workflows/planning.yml` and
  `.github/workflows/ci.yml`
- Small code changes:
  TDD plus targeted unit tests
- Cross-package logic changes:
  unit tests, and integration tests if boundaries changed; use differential mutation
  testing when its extra logic-quality signal justifies the cost
- CLI workflow changes:
  unit tests plus targeted e2e coverage
- Tracked-work system changes:
  workflow validation, skill parity check, and spec verification
- Spec or planning edits:
  `taskrail validate` is a hard CI gate; spec coverage is advisory
- All code changes:
  manual testing against task acceptance criteria before verification
