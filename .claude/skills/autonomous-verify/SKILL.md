---
name: autonomous-verify
description: Run deterministic verification against Tessariq tracked-work acceptance criteria and spec coverage
argument-hint: "[profile]"
---

# autonomous-verify

Run deterministic verification against Tessariq tracked-work acceptance criteria and spec coverage.

## Required Flow

1. Run `taskrail validate`.
2. Choose the verification scope: a specific task, or spec coverage across the backlog.
3. For a task, run `taskrail verify <task-id> --result pass|fail --summary "<s>" [--details "<d>"]`. For spec coverage, run `taskrail coverage --json` (advisory, read-only).
4. Review the reported result and any uncovered spec references.
5. Review unresolved findings.
6. When findings deserve backlog treatment, create a follow-up with `taskrail verify <task-id> --create-followup --followup-title "<t>" --followup-description "<d>" [--followup-priority high|medium|low]`.
7. Run `taskrail repair --apply`.

## Rules

- verification-only runs must not mutate product code unless explicitly requested
- keep mutation testing in mind for logic-heavy changes and CI uses a 70% threshold
- keep evidence paths in reports and notes
