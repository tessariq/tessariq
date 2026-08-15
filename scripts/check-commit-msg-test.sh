#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
checker="$script_dir/check-commit-msg.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_accepts() {
  local name="$1"
  local message="$2"
  local message_file="$tmp_dir/$name"
  local output

  printf '%s\n' "$message" >"$message_file"
  if ! output="$("$checker" "$message_file" 2>&1)"; then
    fail "$name was rejected: $output"
  fi
}

assert_rejects() {
  local name="$1"
  local message="$2"
  local expected="$3"
  local message_file="$tmp_dir/$name"
  local output

  printf '%s\n' "$message" >"$message_file"
  if output="$("$checker" "$message_file" 2>&1)"; then
    fail "$name was accepted"
  fi
  if [[ "$output" != *"$expected"* ]]; then
    fail "$name did not report '$expected': $output"
  fi
}

body_72="$(printf '%072d' 0)"
body_73="$(printf '%073d' 0)"

assert_accepts conventional-without-task $'docs: clarify contributor workflow\n\nExplain why contributors need the clarified path.'
assert_accepts short-task-suffix $'feat(workspace): add copy mode (T-124)\n\nKeep task work isolated when a worktree is unavailable.'
assert_accepts generated-merge "Merge branch 'T-124-copy-mode'"
assert_accepts generated-revert 'Revert "feat: add copy mode (T-124)"'
assert_accepts generated-fixup 'fixup! feat: add copy mode (T-124)'
assert_accepts generated-squash 'squash! feat: add copy mode (T-124)'
assert_accepts generated-prose $'test: describe generated fixtures\n\nFixtures are generated with go generate.'
assert_accepts body-at-72-characters "test: accept bounded body lines

$body_72"

assert_rejects missing-body 'docs: reject missing commit body' 'descriptive body'
assert_rejects unseparated-body $'docs: reject unseparated body\nExplain why this changed.' 'descriptive body'
assert_rejects body-over-72-characters "test: reject long body lines

$body_73" '72 characters'
assert_rejects slugged-task-suffix $'feat: add copy mode (T-124-copy-mode)\n\nExplain the copy-mode change.' 'task references'
assert_rejects prefixed-task $'feat: T-124 add copy mode\n\nExplain the copy-mode change.' 'task references'
assert_rejects misplaced-task $'feat: add T-124 copy mode\n\nExplain the copy-mode change.' 'task references'
assert_rejects mixed-task-references $'feat: T-124 add copy mode (T-125)\n\nExplain the copy-mode change.' 'task references'
assert_rejects multiple-task-suffixes $'feat: add copy mode (T-124) (T-125)\n\nExplain the copy-mode change.' 'task references'
assert_rejects invalid-conventional $'add copy mode (T-124)\n\nExplain the copy-mode change.' 'Conventional Commit'
assert_rejects coauthor-attribution $'feat: add copy mode (T-124)\n\nExplain the copy-mode change.\n\nCo-Authored-By: Example Bot <bot@example.com>' 'automated-attribution'
assert_rejects assisted-attribution $'docs: reject assisted attribution\n\nExplain the documentation change.\n\nAssisted-by: Example Tool' 'automated-attribution'
assert_rejects generated-attribution $'docs: reject generated attribution\n\nExplain the documentation change.\n\nGenerated with Example Tool' 'automated-attribution'

printf 'commit message checks passed\n'
