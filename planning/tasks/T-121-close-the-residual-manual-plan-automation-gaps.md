---
id: T-121-close-the-residual-manual-plan-automation-gaps
title: Close the residual manual-plan automation gaps
status: todo
priority: low
spec_ref: specs/tessariq-v0.1.0.md#acceptance-scenarios
dependencies:
    - T-117
updated_at: "2026-07-31T12:19:42Z"
---

# T-121-close-the-residual-manual-plan-automation-gaps Close the residual manual-plan automation gaps

## Description

Follow-up derived from T-117. Partitioning the v0.1.0 manual test plan surfaced six cases that are mechanically automatable but currently thin. They are recorded in `docs/release-verification.md` section 5 as automation gaps, explicitly not as human-only residue, so a release does not block on them — but they leave real behavior unasserted.

- T2.4: the optional-config warning at `cmd/tessariq/run.go:235` and `:237` is asserted by no test, and it names the agent rather than the unmountable path the manual plan expects. Decide whether the message or the plan expectation is wrong, then assert it.
- T2.7: no end-to-end proof of a successful agent update, or that the resulting version is at least the baked one. Only `--no-update-agent` (`TestE2E_AgentUpdate_SkipFlagBypassesInitPhase`) and the failure fallback (`TestE2E_AgentUpdate_FallbackRecordsEvidence`) are covered.
- T4.2: task-path rejection is unit tested only (`internal/run/taskpath_test.go`); no e2e invokes `tessariq run` with a path outside the repository.
- T4.5: `TestE2E_RunFailsWithActionableGuidanceWhenDockerMissing` removes `docker` from `PATH`; a reachable-but-stopped daemon is a different failure point and is not simulated.
- T4.17: manifest identity tampering is covered for `run_id` and `resolved_egress_mode`; `task_path`, `base_sha`, and task-title trailer fields are not validated by `validateManifestIdentity`.
- T4.24: `version` and `--version` agree in-process (`cmd/tessariq/main_test.go`), but neither is exercised from a non-git directory.

## Acceptance

- Each of the six cases is either covered by a test at the appropriate tier, or the gap row in `docs/release-verification.md` section 5 is updated with why it stays uncovered.
- T2.4 resolves the message-versus-expectation mismatch explicitly rather than asserting the current text by accident.
- T4.17 states whether `task_path`, `base_sha`, and trailer fields are meant to be tamper-checked; if they are, `validateManifestIdentity` covers them and a test proves rejection before any git side effect.
- `docs/release-verification.md` section 5 is left accurate: rows for closed gaps are removed, not just annotated.
- Test tiers follow the pyramid — prefer unit and integration; add e2e only where the case is a CLI flow.

## Verification Notes

- Record evidence paths under `planning/artifacts/verify/<task-id>/`.

## Implementation Notes
