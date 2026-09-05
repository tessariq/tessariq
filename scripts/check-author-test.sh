#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
checker="$script_dir/check-author.sh"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_accepts() {
  local identity="$1"
  local output

  if ! output="$(bash "$checker" "$identity" 2>&1)"; then
    fail "$identity was rejected: $output"
  fi
}

assert_rejects() {
  local identity="$1"
  local output

  if output="$(bash "$checker" "$identity" 2>&1)"; then
    fail "$identity was accepted"
  fi
  if [[ "$output" != *"agent identity"* ]]; then
    fail "$identity did not report the policy: $output"
  fi
}

assert_accepts 'Felix Müller <felix@fmueller.io>'
assert_accepts 'Amparo Ruiz <amparo@example.com>'
assert_accepts 'Bottger Lang <bottger@example.com>'

assert_rejects 'Amp <amp@ampcode.com>'
assert_rejects 'Someone <someone@ampcode.com>'
assert_rejects 'Claude <noreply@anthropic.com>'
assert_rejects 'Claude Opus 5 <claude@example.com>'
assert_rejects 'Cursor Agent <agent@cursor.com>'

if output="$(
  GIT_AUTHOR_NAME=Fixture GIT_AUTHOR_EMAIL=fixture@example.com \
    GIT_COMMITTER_NAME=Amp GIT_COMMITTER_EMAIL=amp@ampcode.com \
    bash "$checker" 2>&1
)"; then
  fail "an agent committer was accepted"
fi
if [[ "$output" != *"agent identity"* ]]; then
  fail "the committer check did not report the policy: $output"
fi

printf 'author checks passed\n'
