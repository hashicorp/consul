#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# check-jira-ids.sh
#
# Detects internal Jira ticket IDs (e.g. CCT-123, NET-456) in two places:
#
#   1. PR description  — read from the JIRA_CHECK_PR_BODY environment variable
#      (the raw text of github.event.pull_request.body passed in by the caller).
#
#   2. Lines added by the PR — for each file in the changed-files list, only
#      the lines introduced by this PR (the '+' lines of the git diff between
#      BASE_SHA and HEAD_SHA) are scanned.  Pre-existing Jira IDs in unchanged
#      surrounding context are intentionally ignored; they pre-date this policy
#      and would produce false positives on routine file modifications.
#
# A Jira ID is defined as:   [A-Z]{2,10}-[0-9]+
#
# Excluded project keys:
#   Keys that collide with common CE terminology (SPDX license IDs, protocol
#   acronyms, etc.) are listed in EXCLUDE_KEYS below and stripped before the
#   check runs.
#
# Usage:
#   BASE_SHA=<base> HEAD_SHA=<head> \
#   JIRA_CHECK_PR_BODY="<pr body text>" \
#     ./check-jira-ids.sh <path-to-file-listing-changed-paths>
#
# Environment:
#   BASE_SHA             Required. The merge-base / PR base commit SHA.
#   HEAD_SHA             Required. The PR head commit SHA.
#   JIRA_CHECK_PR_BODY   Raw text of the PR description (may be empty/unset).
#
# Exit codes:
#   0  No Jira IDs found in either surface.
#   1  One or more Jira IDs detected; prints GitHub Actions error annotations.

set -euo pipefail

CHANGED_FILES_LIST="${1:-}"

if [[ -z "$CHANGED_FILES_LIST" ]]; then
  echo "Usage: BASE_SHA=<sha> HEAD_SHA=<sha> $0 <file-listing-changed-paths>" >&2
  exit 1
fi

if [[ ! -f "$CHANGED_FILES_LIST" ]]; then
  echo "Error: file not found: ${CHANGED_FILES_LIST}" >&2
  exit 1
fi

BASE_SHA="${BASE_SHA:-}"
HEAD_SHA="${HEAD_SHA:-}"

if [[ -z "$BASE_SHA" || -z "$HEAD_SHA" ]]; then
  echo "Error: BASE_SHA and HEAD_SHA must be set." >&2
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Compound tokens to strip before per-key exclusion.
#
# Multi-word technical phrases whose fragments would match the Jira-ID pattern
# after per-key stripping.  Strip the whole phrase first so neither piece is
# misidentified.
#
# Example:
#   "TEST-NET-1" (RFC 5735 reserved network block) — strip "TEST-NET" as a
#   unit; without this, "NET-1" would remain after "TEST-" is consumed.
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_COMPOUND_TOKENS=(
  'TEST-NET'   # RFC 5735 documentation address blocks (TEST-NET-1/2/3)
)

# ─────────────────────────────────────────────────────────────────────────────
# Project keys to exclude.
#
# Add any uppercase prefix that appears as a legitimate code token in CE source
# and matches the Jira ID pattern when followed by a dash and digits.
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_KEYS=(
  # SPDX license identifiers — present in every file's copyright header
  'BUSL'
  'MPL'
  # Well-known technical acronyms
  'HTTP'
  'SHA'
  'RFC'
  'TLS'
  'UTF'
  'SSL'
  'DNS'
  'TCP'
  'UDP'
  'AES'
  'RSA'
  'URI'
  'URL'
  'API'
)

# ─────────────────────────────────────────────────────────────────────────────
# File types to scan (mirrors check-ent-content-markers.sh for consistency).
# ─────────────────────────────────────────────────────────────────────────────
SCANNABLE_PATTERN='\.(go|yaml|yml|hcl|tf|sh|bash|mod|json|toml|makefile|mk|md|txt)$|/VERSION$|^VERSION$'
EXCLUDE_DIRS_PATTERN='^(vendor/|\.git/|node_modules/)'

# ─────────────────────────────────────────────────────────────────────────────
# Paths to exclude from the file scan.
#
# The Jira-check script itself necessarily contains the Jira ID pattern as a
# regex literal — scanning it produces false positives.
# CHANGELOG.md may legitimately reference internal trackers in older entries
# that pre-date this policy.
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_PATHS=(
  '.github/scripts/check-jira-ids.sh'
  'CHANGELOG.md'
)

# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

# build_exclude_sed_expr — sed expression that strips compound tokens first,
# then per-key tokens, so fragments of compound phrases are never left behind.
build_exclude_sed_expr() {
  local expr=""
  for compound in "${EXCLUDE_COMPOUND_TOKENS[@]}"; do
    expr+="s/${compound}-[0-9][0-9]*/EXCLUDED/g;"
  done
  for key in "${EXCLUDE_KEYS[@]}"; do
    expr+="s/${key}-[0-9][0-9]*/EXCLUDED/g;"
  done
  printf '%s' "$expr"
}

SED_EXCLUDE_EXPR="$(build_exclude_sed_expr)"

# find_jira_ids_in_text <text> — prints one match per line, empty = none.
find_jira_ids_in_text() {
  printf '%s' "$1" \
    | sed -E "$SED_EXCLUDE_EXPR" \
    | grep -oE '[A-Z]{2,10}-[0-9]+' \
    || true
}

is_excluded_path() {
  local file="$1"
  for excl in "${EXCLUDE_PATHS[@]}"; do
    [[ "$file" == *"$excl"* ]] && return 0
  done
  return 1
}

# ─────────────────────────────────────────────────────────────────────────────
# Surface 1 — PR description
# ─────────────────────────────────────────────────────────────────────────────
violations=0
pr_body="${JIRA_CHECK_PR_BODY:-}"

if [[ -n "$pr_body" ]]; then
  pr_hits="$(find_jira_ids_in_text "$pr_body")"
  if [[ -n "$pr_hits" ]]; then
    while IFS= read -r id; do
      [[ -z "$id" ]] && continue
      echo "::error::Jira ID '${id}' found in PR description. Internal ticket references must not appear in community PRs."
      violations=$((violations + 1))
    done <<< "$pr_hits"
  else
    echo "PR description: no Jira IDs found." >&2
  fi
else
  echo "PR description: empty or not provided; skipping description check." >&2
fi

# ─────────────────────────────────────────────────────────────────────────────
# Surface 2 — lines added by this PR (diff only, not full file contents)
# ─────────────────────────────────────────────────────────────────────────────
scanned=0

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  [[ ! -f "$file" ]] && continue

  if [[ "$file" =~ $EXCLUDE_DIRS_PATTERN ]]; then
    continue
  fi

  if ! [[ "$(printf '%s' "$file" | tr '[:upper:]' '[:lower:]')" =~ $SCANNABLE_PATTERN ]]; then
    continue
  fi

  if is_excluded_path "$file"; then
    echo "Skipping excluded file: ${file}" >&2
    continue
  fi

  scanned=$((scanned + 1))

  # Extract only lines introduced by this PR ('+' lines, excluding the '+++'
  # diff header).  Line numbers in error annotations refer to the position
  # within the added-lines output, not the final file — sufficient to locate
  # the violation for the author.
  added_lines="$(git diff "${BASE_SHA}" "${HEAD_SHA}" -- "${file}" \
    | grep '^+' \
    | grep -v '^+++' \
    | sed 's/^+//' \
    || true)"

  [[ -z "$added_lines" ]] && continue

  file_hits="$(printf '%s\n' "$added_lines" \
    | sed -E "$SED_EXCLUDE_EXPR" \
    | grep -onE '[A-Z]{2,10}-[0-9]+' \
    || true)"

  if [[ -n "$file_hits" ]]; then
    while IFS=: read -r lineno id; do
      [[ -z "$id" ]] && continue
      echo "::error file=${file},line=${lineno}::Jira ID '${id}' introduced in PR. Internal ticket references must not appear in CE source."
      violations=$((violations + 1))
    done <<< "$file_hits"
  fi

done < "$CHANGED_FILES_LIST"

echo "" >&2
echo "Files scanned : ${scanned}" >&2
echo "Violations    : ${violations}" >&2

if [[ "$violations" -gt 0 ]]; then
  echo "::error::Jira ID check failed with ${violations} violation(s)."
  echo "::error::Internal Jira ticket references must not appear in PR descriptions or CE source files."
  echo "::error::Remove or replace them with public GitHub issue links before merging."
  exit 1
fi

echo "Jira ID check passed: no internal ticket references found in PR changes."
