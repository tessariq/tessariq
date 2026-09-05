#!/usr/bin/env bash
# Refuse commits whose author or committer identifies an automated agent. The
# optional argument makes the same policy reusable for history scans.
set -euo pipefail

check_identity() {
  local identity="$1"

  if printf '%s' "$identity" | grep -qiE '<[^>]*@(ampcode\.com|anthropic\.com|openai\.com|cursor\.(com|sh)|devin\.ai)>' \
    || printf '%s' "$identity" | grep -qiE '^(amp|claude|codex|copilot|cursor|devin|agent|bot)([[:space:]]|<|-)' \
    || printf '%s' "$identity" | grep -qiE '<(amp|claude|codex|copilot|cursor|devin)(-?agent)?@'; then
    echo "check-author: refusing an agent identity:" >&2
    echo "  $identity" >&2
    echo "set user.name and user.email to a person before committing" >&2
    exit 1
  fi
}

if [ -n "${1:-}" ]; then
  check_identity "$1"
else
  check_identity "$(git var GIT_AUTHOR_IDENT | sed -E 's/> [0-9]+ [+-][0-9]{4}$/>/')"
  check_identity "$(git var GIT_COMMITTER_IDENT | sed -E 's/> [0-9]+ [+-][0-9]{4}$/>/')"
fi
