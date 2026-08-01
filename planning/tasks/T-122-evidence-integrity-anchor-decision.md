---
id: T-122-evidence-integrity-anchor-decision
title: Decide whether evidence needs an integrity anchor outside .tessariq/runs
status: todo
priority: low
spec_ref: specs/tessariq-v0.2.0.md#evidence-additions
dependencies:
    - T-121-close-the-residual-manual-plan-automation-gaps
updated_at: "2026-08-01T08:04:53Z"
---

# T-122-evidence-integrity-anchor-decision Decide whether evidence needs an integrity anchor outside .tessariq/runs

## Description

**This is a decision task, not an implementation commitment.** The outcome may well be
"documented as out of scope, no code". Do not start building a signing scheme without
first landing the decision.

Surfaced by the security review of T-121. Tessariq's promote-time tamper checks all
compare evidence files against other evidence files:

- `internal/promote/promote.go` — `validateManifestIdentity` checks `manifest.json`
  `run_id`/`task_path`/`task_title` against the run index entry and `base_sha` against
  `workspace.json`
- `internal/runner/completeness.go` — `CheckEvidenceCompleteness` checks
  `manifest.json` `resolved_egress_mode` against `runtime.json`

Every source lives under `.tessariq/runs/` with the same `0700`/`0600` permissions, is
written by the same host-side process, and carries no signature or hash chain. So these
are **consistency checks, not an integrity boundary**: they catch partial edits and
evidence-writing bugs, but not an actor who rewrites every file coherently. Two concrete
weaknesses the review named:

- Anyone who can forge `manifest.json` can equally edit `workspace.json` and
  `runtime.json` in the same pass.
- `index.jsonl` is append-only in form with no dedup or immutability enforcement, and
  both resolution paths take the *most recent* matching entry
  (`internal/run/runref.go` — `uniqueRuns` keeps the last entry per `run_id`,
  `resolveByID` scans from the end). Appending one forged line with the target `run_id`
  is a lower-effort bypass than editing anything in place.

T-121 scoped the claim honestly in `CHANGELOG.md` and in the `validateManifestIdentity`
doc comment rather than overstating it. This task decides whether v0.2.0 should do more.

Context that matters for the decision: the threat model is unclear. Evidence sits inside
the user's own repository, under their own uid, and the promoter is the same person who
ran the agent. A signing key stored on the same machine buys little; the realistic
attacker may be a compromised agent writing into the worktree, a malicious `diff.patch`,
or a shared/CI checkout — not a local user editing their own JSON. Establish which of
these, if any, is in scope *before* weighing mechanisms.

## Acceptance

- The threat model is written down explicitly: who the adversary is, what access they
  have, and what a successful attack looks like. If no adversary in that model is
  actually defeated by an integrity anchor, say so and stop there.
- A decision is recorded — implement, defer, or decline — with the reasoning, in
  `specs/tessariq-v0.2.0.md` (evidence additions) if it changes the spec, otherwise in
  `docs/`.
- If the decision is to implement, the mechanism is named concretely (what is hashed or
  signed, where the anchor lives such that the same actor cannot rewrite it, how key
  material is handled, and what `promote` does on verification failure), and follow-up
  implementation tasks are created with `taskrail task new`. This task itself stays a
  decision task.
- If the decision is to decline or defer, the existing "consistency check, not an
  integrity boundary" wording in `internal/promote/promote.go`, `CHANGELOG.md` and the
  v0.1.0 release notes is confirmed as still accurate, and the `index.jsonl`
  last-entry-wins bypass is recorded as known and accepted.
- No behavior change is required by this task.

## Verification Notes

- Record evidence paths under `planning/artifacts/verify/<task-id>/`.
- The deliverable is a written decision. Verification means the decision document exists,
  states the threat model, and matches whatever the code and changelog claim.

## Implementation Notes

- Prior art to review before deciding: in-toto attestations, SLSA provenance, Sigstore
  keyless signing, and `git notes` as an anchor that lives in the object database rather
  than the working tree.
- Cheap partial measures worth pricing against a full signing scheme: making `index.jsonl`
  append-only in enforcement (reject a second entry for a `run_id` whose state is already
  terminal), or recording an evidence digest in the promote commit trailer so a
  *promoted* run becomes verifiable after the fact even if the evidence tree does not.
