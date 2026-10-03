---
id: T-125-structured-logging-decision
title: Decide whether the CLI needs structured diagnostic logging
status: todo
priority: low
spec_ref: specs/tessariq-v0.2.0.md#failure-ux
dependencies: []
updated_at: "2026-10-03T13:56:16Z"
---

# T-125-structured-logging-decision Decide whether the CLI needs structured diagnostic logging

## Description

**This is a decision task, not an implementation commitment.** The outcome may
be "no logger; keep the current approach", with nothing to build.

`AGENTS.md` used to say "Use structured logging with `zap` for diagnostics" and
"Log Docker and git operations at debug level; surface failures at error
level". Neither line matched the code, so both were removed. Nothing in the
module imports `zap` or any other logger, and the guidance came from the
initial scaffold commit (`0ae6014`). It also conflicted with the "prefer the
standard library first; add dependencies only when justified" rule.

What exists today:

- User-facing output goes through command writers (`cmd.OutOrStdout()` /
  stderr). Failures surface as wrapped errors returned to the CLI.
- Run diagnostics are written as evidence files (`run.log`, `runner.log`,
  optional `squid.log`) per the v0.1.0 spec's evidence contract.
- No `--debug`/`--verbose` flag or debug output path exists for the Docker
  and git commands the CLI shells out to.

The open question is whether operators need host-side diagnostics beyond
evidence files and error messages, for example to debug a failed
`docker create` or `git worktree add` without re-running by hand.

## Acceptance

- A decision is recorded (adopt a logger, add a narrower debug mechanism, or
  decline) with the reasoning, in `docs/` or in `specs/tessariq-v0.2.0.md`
  (Failure UX) if it changes user-visible behavior.
- If adopting, the choice is justified against the stdlib-first rule. Weigh
  `log/slog` first; a third-party logger such as `zap` needs a concrete reason
  `slog` cannot meet. Also decide:
  - how it is enabled (flag, env var)
  - where output goes (stderr vs. evidence)
  - how it stays out of stdout, which carries run IDs and evidence paths for
    scripting
- If adopting, the `AGENTS.md` "User output" section regains a logging rule
  that matches the chosen mechanism, and follow-up implementation tasks are
  created with `taskrail task new`. This task itself stays a decision task.
- If declining, `AGENTS.md` stays as it is (no logging guidance) and the
  decision note says so.
- No behavior change is required by this task.

## Verification Notes

- Record evidence paths under `planning/artifacts/verify/<task-id>/`.
- The deliverable is a written decision; verification means it exists and
  matches `AGENTS.md`.

## Implementation Notes

- Check how `runner.log` is written today (`internal/runner/`) before
  proposing a second diagnostics channel. Extending evidence logging may
  cover the need.
