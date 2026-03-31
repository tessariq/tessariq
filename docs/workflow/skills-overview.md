# Skills Overview

Skill catalog for deterministic tracked-work execution in Tessariq.

## Canonical Skills

- `autonomous-backlog`
- `autonomous-manual-test`
- `autonomous-recovery`
- `autonomous-task`
- `autonomous-verify`
- `autonomous-verify-hybrid`

## Packaging

- Canonical workflow guidance lives in `docs/workflow/`.
- Mirrored skill files live in `.agents/skills/` and `.claude/skills/`.
- The mirrored skill directories must stay byte-identical and are checked by CI.

## Required Behavior

- skills must route all state transitions through `go run ./cmd/tessariq-workflow ...`
- implementation skills must enforce TDD
- all skills must respect the testing pyramid
- integration and e2e guidance must require Testcontainers for Go and reject custom local servers
- verification guidance must mention mutation testing and the 70% threshold
- implementation-task guidance must require one conventional-commit commit per task (no separate implementation vs workflow-update commits)
