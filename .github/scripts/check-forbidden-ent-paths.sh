#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# check-forbidden-ent-paths.sh
#
# Scans a newline-delimited list of changed paths and fails if any path matches
# a forbidden ENT-only naming convention that must never appear in the CE repo.
#
# Usage:
#   ./check-forbidden-ent-paths.sh <path-to-file-listing-changed-paths>
#
# Exit codes:
#   0  No forbidden ENT-style paths found
#   1  One or more forbidden ENT-style paths detected, or invalid usage

set -euo pipefail

CHANGED_FILES_LIST="${1:-}"
FORBIDDEN_PATH_PATTERN='(^|/)[^/]+_ent(_test)?\.go$|(^|/)[^/]+_enterprise\.go$|(^|/)(enterprise|ent)/'

if [[ -z "$CHANGED_FILES_LIST" ]]; then
  echo "Usage: $0 <file-listing-changed-paths>" >&2
  exit 1
fi

if [[ ! -f "$CHANGED_FILES_LIST" ]]; then
  echo "Error: file not found: ${CHANGED_FILES_LIST}" >&2
  exit 1
fi

violations=$(grep -E "$FORBIDDEN_PATH_PATTERN" "$CHANGED_FILES_LIST" || true)
violations=$(printf '%s\n' "$violations" | sed '/^$/d' | sort -u)

if [[ -z "$violations" ]]; then
  echo "Forbidden ENT-style path check passed: no violations found." >&2
  exit 0
fi

while IFS= read -r path; do
  [[ -z "$path" ]] && continue
  echo "$path"
  echo "::error file=${path}::Forbidden ENT-style path detected in CE repository changeset." >&2
done <<< "$violations"

exit 1