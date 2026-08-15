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

assert_accepts conventional-without-task 'docs: clarify contributor workflow'
assert_accepts short-task-suffix 'feat(workspace): add copy mode (T-124)'
assert_accepts generated-merge "Merge branch 'T-124-copy-mode'"
assert_accepts generated-revert 'Revert "feat: add copy mode (T-124)"'
assert_accepts generated-fixup 'fixup! feat: add copy mode (T-124)'
assert_accepts generated-squash 'squash! feat: add copy mode (T-124)'

assert_rejects slugged-task-suffix 'feat: add copy mode (T-124-copy-mode)' 'task references'
assert_rejects prefixed-task 'feat: T-124 add copy mode' 'task references'
assert_rejects misplaced-task 'feat: add T-124 copy mode' 'task references'
assert_rejects invalid-conventional 'add copy mode (T-124)' 'Conventional Commit'
assert_rejects bot-attribution $'feat: add copy mode (T-124)\n\nCo-Authored-By: Example Bot <bot@example.com>' 'automated-attribution'

printf 'commit message checks passed\n'
