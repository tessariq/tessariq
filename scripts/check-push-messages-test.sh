#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
scanner="$script_dir/check-push-messages.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

# Build history without hooks to model commits created with --no-verify or by
# commands such as rebase that do not invoke commit-msg.
repo="$tmp_dir/repo"
git init -q "$repo"
cd "$repo"
git config --local core.hooksPath .git/hooks
git config user.email fixture@example.com
git config user.name Fixture
git config commit.gpgsign false
export GIT_AUTHOR_NAME=Fixture
export GIT_AUTHOR_EMAIL=fixture@example.com
export GIT_COMMITTER_NAME=Fixture
export GIT_COMMITTER_EMAIL=fixture@example.com

commit() {
  printf '%s\n' "$2" >>file.txt
  git add file.txt
  git commit -q --no-verify -m "$1"
}

commit $'feat: add the first fact\n\nExplain the change.' one
clean_head="$(git rev-parse HEAD)"
if ! output="$(bash "$scanner" "$clean_head" 2>&1)"; then
  fail "a clean commit was rejected: $output"
fi

commit $'feat: add the second fact\n\nExplain the change.\n\nAmp-Thread:\nhttps://ampcode.com/threads/T-01234567-89ab-cdef-0123-456789abcdef' two
if output="$(bash "$scanner" "$clean_head..HEAD" 2>&1)"; then
  fail "a session link was accepted"
fi
if [[ "$output" != *"automated-attribution"* ]]; then
  fail "the scanner did not name the policy: $output"
fi

# Git passes four fields per ref update to pre-push on stdin.
if printf 'refs/heads/main %s refs/heads/main %s\n' "$(git rev-parse HEAD)" "$clean_head" | bash "$scanner" >/dev/null 2>&1; then
  fail "the stdin range accepted a session link"
fi
if ! printf 'refs/heads/main %s refs/heads/main %s\n' "$clean_head" "$clean_head" | bash "$scanner" >/dev/null 2>&1; then
  fail "the stdin range rejected an empty push"
fi

commit $'feat: add the third fact\n\nExplain the change.' three
agent_parent="$(git rev-parse HEAD~1)"
git commit -q --amend --no-verify --no-edit --author='Amp <amp@ampcode.com>'
if output="$(bash "$scanner" "$agent_parent..HEAD" 2>&1)"; then
  fail "an agent author was accepted"
fi
if [[ "$output" != *"agent identity"* ]]; then
  fail "the scanner did not name the author policy: $output"
fi

commit $'feat: add the fourth fact\n\nExplain the change.' four
committer_parent="$(git rev-parse HEAD~1)"
GIT_COMMITTER_NAME=Amp GIT_COMMITTER_EMAIL=amp@ampcode.com \
  git commit -q --amend --no-verify --no-edit
if output="$(bash "$scanner" "$committer_parent..HEAD" 2>&1)"; then
  fail "an agent committer was accepted"
fi
if [[ "$output" != *"agent identity"* ]]; then
  fail "the scanner did not name the committer policy: $output"
fi

printf 'push message checks passed\n'
