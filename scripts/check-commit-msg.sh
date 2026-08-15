#!/usr/bin/env bash
# commit-msg hook: enforce a Conventional Commit subject and descriptive body,
# normalize task references, and reject automated-attribution trailers.
#
# Usage: scripts/check-commit-msg.sh <commit-msg-file>
#
# Mirrors the repo commit conventions (see AGENTS.md): the subject is a
# Conventional Commit, includes context beyond the subject, uses short-key task
# suffixes, and carries no automated-attribution lines. Exits non-zero with a
# clear, quotable message on failure.
set -euo pipefail

msg_file="${1:-}"
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "check-commit-msg: missing commit message file argument" >&2
  exit 1
fi

# Subject = first non-comment, non-empty line.
subject="$(grep -vE '^[[:space:]]*#' "$msg_file" | sed '/^[[:space:]]*$/d' | head -n 1)"
require_body=true

case "$subject" in
  "Merge "* | "Revert "* | "fixup! "* | "squash! "*)
    require_body=false
    ;;
  *)
    if ! printf '%s' "$subject" | grep -qE '^(feat|fix|refactor|docs|test|chore|perf|ci)(\([a-z0-9._-]+\))?!?: .+'; then
      echo "check-commit-msg: subject must be a Conventional Commit:" >&2
      echo "  <type>: <description>   (types: feat fix refactor docs test chore perf ci)" >&2
      echo "got: ${subject:-<empty>}" >&2
      exit 1
    fi
    if printf '%s' "$subject" | grep -qE 'T-[0-9]+' \
      && ! printf '%s' "$subject" | grep -qE '\(T-[0-9]+\)$'; then
      echo "check-commit-msg: task references must use the short key as a subject suffix:" >&2
      echo "  feat: add copy mode (T-124)" >&2
      echo "not a prefix, misplaced key, or full slugged task identifier" >&2
      exit 1
    fi
    subject_without_task_suffix="$(printf '%s' "$subject" | sed -E 's/ \(T-[0-9]+\)$//')"
    if printf '%s' "$subject_without_task_suffix" | grep -qE 'T-[0-9]+'; then
      echo "check-commit-msg: task references must use the short key as a subject suffix:" >&2
      echo "  feat: add copy mode (T-124)" >&2
      echo "not a prefix, misplaced key, or additional task reference" >&2
      exit 1
    fi
    ;;
esac

if [[ "$require_body" == true ]]; then
  if ! awk '
    /^[[:space:]]*#/ { next }
    !subject && /^[[:space:]]*$/ { next }
    !subject { subject = 1; next }
    !separator && /^[[:space:]]*$/ { separator = 1; next }
    !separator { invalid = 1; next }
    !/^[[:space:]]*$/ { body = 1 }
    END { exit !(separator && body && !invalid) }
  ' "$msg_file"; then
    echo "check-commit-msg: add a descriptive body after the subject" >&2
    echo "  After a blank line, explain the commit's intent, context, and non-obvious decisions." >&2
    exit 1
  fi

  if ! awk '
    /^[[:space:]]*#/ { next }
    !subject && /^[[:space:]]*$/ { next }
    !subject { subject = 1; next }
    !separator && /^[[:space:]]*$/ { separator = 1; next }
    separator && length($0) > 72 { exit 1 }
  ' "$msg_file"; then
    echo "check-commit-msg: wrap body lines at 72 characters" >&2
    exit 1
  fi
fi

# Reject automated-attribution lines.
if grep -qiE '^[[:space:]]*((co-authored-by|assisted-by):|generated with[[:space:]])' "$msg_file" \
  || grep -qF '🤖' "$msg_file"; then
  echo "check-commit-msg: remove automated-attribution lines" >&2
  echo "  (Co-authored-by: / Assisted-by: / 'Generated with ...' / bot marker) - attribution is disabled for this repo." >&2
  exit 1
fi

exit 0
