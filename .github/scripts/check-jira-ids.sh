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
#   2. Changed source / config files — a newline-delimited list of paths is
#      passed as the first positional argument, identical to the calling
#      convention used by check-ent-content-markers.sh.
#
# A Jira ID is defined as:   [A-Z]{2,10}-[0-9]+
# … preceded by a word boundary (start-of-string, whitespace, punctuation) and
# followed by a non-digit boundary, to avoid matching semver strings like
# "1.2-3" or hex SHAs.  The pattern is intentionally broad; if a project key
# collision with a legitimate CE term is discovered, add it to EXCLUDE_KEYS.
#
# Excluded project keys:
#   Keys that collide with common CE terminology (Go package paths, acronyms,
#   etc.) are listed in EXCLUDE_KEYS below and stripped before the check runs.
#
# Usage:
#   JIRA_CHECK_PR_BODY="<pr body text>" \
#     ./check-jira-ids.sh <path-to-file-listing-changed-paths>
#
# Environment:
#   JIRA_CHECK_PR_BODY   Raw text of the PR description (may be empty/unset).
#
# Exit codes:
#   0  No Jira IDs found in either surface.
#   1  One or more Jira IDs detected; prints GitHub Actions error annotations.

set -euo pipefail

CHANGED_FILES_LIST="${1:-}"

if [[ -z "$CHANGED_FILES_LIST" ]]; then
  echo "Usage: $0 <file-listing-changed-paths>" >&2
  exit 1
fi

if [[ ! -f "$CHANGED_FILES_LIST" ]]; then
  echo "Error: file not found: ${CHANGED_FILES_LIST}" >&2
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Jira ID pattern:  one or more uppercase letters (2–10), dash, one or more
# digits.  Word-boundary anchors prevent matching inside longer tokens.
# The negative lookbehind on digits stops matching "1.19-2" semver fragments.
# ─────────────────────────────────────────────────────────────────────────────
JIRA_PATTERN='(^|[^A-Z0-9])([A-Z]{2,10}-[0-9]+)([^0-9]|$)'

# ─────────────────────────────────────────────────────────────────────────────
# Project keys to exclude.
#
# Add any uppercase two-to-ten-letter prefix that:
#   • appears as a legitimate code token or known acronym in CE source, AND
#   • matches the Jira ID pattern when followed by a dash and digits.
#
# Examples:
#   HTTP-1  → false positive in "HTTP-1.1" headers documentation
#   SHA-256, SHA-512 → hash algorithm names
#   RFC-2119 → standards reference
#   TLS-1, TLS-13 → protocol version numbers
#   UTF-8, UTF-16 → encoding names
#   IPv4, IPv6 would not match (digits in key are excluded by {2,10} cap)
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_KEYS=(
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
# The Jira-check scripts and workflows themselves necessarily contain the Jira
# ID pattern as a regex literal — scanning them produces false positives.
# CHANGELOG.md may legitimately reference internal issue trackers in older
# entries that pre-date this policy.
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_PATHS=(
  '.github/scripts/check-jira-ids.sh'
  '.github/workflows/jira-id-check.yml'
  'CHANGELOG.md'
)

# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

# build_exclude_sed_expr — constructs a sed expression that deletes any token
# matching an excluded key from a line of text before the Jira pattern is
# applied.  This prevents e.g. "SHA-256" from triggering a false positive
# while still catching "CCT-123" in the same line.
build_exclude_sed_expr() {
  local expr=""
  for key in "${EXCLUDE_KEYS[@]}"; do
    expr+="s/${key}-[0-9][0-9]*/EXCLUDED/g;"
  done
  printf '%s' "$expr"
}

SED_EXCLUDE_EXPR="$(build_exclude_sed_expr)"

# strip_excluded <text> — removes excluded-key tokens from text before matching.
strip_excluded() {
  printf '%s' "$1" | sed -E "$SED_EXCLUDE_EXPR"
}

# find_jira_ids_in_text <text> — prints one match per line, empty output = none.
find_jira_ids_in_text() {
  local text="$1"
  strip_excluded "$text" \
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
# Surface 2 — changed files
# ─────────────────────────────────────────────────────────────────────────────
scanned=0

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  [[ ! -f "$file" ]] && continue

  if [[ "$file" =~ $EXCLUDE_DIRS_PATTERN ]]; then
    continue
  fi

  if ! [[ "${file,,}" =~ $SCANNABLE_PATTERN ]]; then
    continue
  fi

  if is_excluded_path "$file"; then
    echo "Skipping excluded file: ${file}" >&2
    continue
  fi

  scanned=$((scanned + 1))

  # Read the file, strip excluded key tokens, then grep for Jira IDs.
  file_hits="$(sed -E "$SED_EXCLUDE_EXPR" "$file" \
    | grep -onE '[A-Z]{2,10}-[0-9]+' \
    || true)"

  if [[ -n "$file_hits" ]]; then
    while IFS=: read -r lineno id; do
      [[ -z "$id" ]] && continue
      echo "::error file=${file},line=${lineno}::Jira ID '${id}' found in changed file. Internal ticket references must not appear in CE source."
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

echo "Jira ID check passed: no internal ticket references found."
