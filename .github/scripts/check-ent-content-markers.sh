#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# check-ent-content-markers.sh
#
# Scans a list of changed files for ENT (enterprise) specific content markers
# that must not appear in a CE (community edition) repository.
#
# The default marker patterns cover:
#   - Go enterprise build constraints  (consul: //go:build consulent)
#   - IBM copyright headers            (crt-core-helloworld ENT-owned files)
#   - ENT-only Go package imports      (go-licensing, etc.)
#   - ENT CI / build metadata          (VERSION_METADATA: "ent", +ent suffix)
#   - Explicit enterprise-repo references in source code
#
# Custom patterns can be added per-repo by setting ENT_MARKER_PATTERNS_FILE to
# the path of a newline-delimited file of extended-regex patterns (blank lines
# and lines starting with '#' are ignored).
#
# Usage:
#   ./check-ent-content-markers.sh <path-to-file-listing-changed-paths>
#
# Environment:
#   ENT_MARKER_PATTERNS_FILE   Optional path to an additional patterns file.
#
# Exit codes:
#   0  No ENT markers found
#   1  One or more ENT markers detected (prints GitHub Actions error annotations)
#
# Extending to consul / consul-enterprise:
#   The //go:build consulent pattern is already included. Add any product-
#   specific patterns to ENT_MARKER_PATTERNS_FILE at runtime, or extend the
#   DEFAULT_PATTERNS array in a fork of this script.

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
# Default ENT content marker patterns (extended regex, case-sensitive).
#
# Each entry is an ERE pattern checked via: grep -E "<pattern>" <file>
# A match in any changed source/config file causes the check to fail.
#
# Pattern rationale:
#   consulent / enterprise go:build  — Go build constraints that gate ENT code
#   Copyright IBM Corp.              — IBM copyright present in ENT-owned Go files
#   go-licensing import              — HashiCorp enterprise licensing package
#   crt-core-helloworld-enterprise   — Direct reference to the ENT repo in source
#   consul-enterprise                — Direct reference to consul ENT in source
#   VERSION_METADATA.*ent            — YAML/HCL CI metadata that sets ENT suffix
#   \+ent in version strings         — +ent version suffix in HCL/YAML config
# ─────────────────────────────────────────────────────────────────────────────
declare -a DEFAULT_PATTERNS=(
  # Go enterprise build constraints (new-style and legacy +build)
  '//go:build consulent'
  '//go:build enterprise'
  '// \+build consulent'
  '// \+build enterprise'

  # ENT-only Go package imports
  '"github\.com/hashicorp/go-licensing'

  # Explicit reference to the enterprise repository in source / configs
  # (does NOT match metadata files that are intentionally about ENT — those
  #  live in .release/ and are excluded via EXCLUDE_PATHS below)
  'crt-core-helloworld-enterprise'

  # consul-enterprise reference inside Go / shell source
  'consul-enterprise'

  # CI / build metadata that stamps the ENT version suffix
  'VERSION_METADATA[[:space:]]*:[[:space:]]*["\x27]ent["\x27]'

  # +ent version suffix in HCL / YAML build configs (e.g. version = "1.2.3+ent")
  '\+ent[[:space:]"'\''`]'
)

# ─────────────────────────────────────────────────────────────────────────────
# Paths to exclude from the content-marker scan.
#
# These are files in the CE repo that legitimately reference ENT concepts
# (e.g., release metadata that records where ENT artifacts are published).
# Paths are matched as prefixes/substrings against the relative file path.
# ─────────────────────────────────────────────────────────────────────────────
declare -a EXCLUDE_PATHS=(
  '.release/ent-release-metadata.hcl'          # CE-side ENT release metadata (intentional)
  'CHANGELOG.md'                                # Changelogs may mention enterprise features

  # The ENT-protection system scripts and workflows necessarily contain ENT
  # marker strings as pattern literals, comments, and documentation.
  # Scanning them produces false positives; exclude the whole group.
  '.github/scripts/check-ent-content-markers.sh'
  '.github/workflows/ent-protection.yml'
  '.github/workflows/sha-ancestry-check.yml'
)

# ─────────────────────────────────────────────────────────────────────────────
# File extensions eligible for scanning.
# Binary files, generated protobuf stubs, and vendor directories are excluded
# by the path filter below; this is an additional type-based safety net.
# ─────────────────────────────────────────────────────────────────────────────
SCANNABLE_PATTERN='\.(go|yaml|yml|hcl|tf|sh|bash|mod|json|toml|makefile|mk)$|/VERSION$|^VERSION$'
EXCLUDE_DIRS_PATTERN='^(vendor/|\.git/|node_modules/)'

# ─────────────────────────────────────────────────────────────────────────────
# Merge optional custom patterns
# ─────────────────────────────────────────────────────────────────────────────
declare -a patterns=("${DEFAULT_PATTERNS[@]}")

if [[ -n "${ENT_MARKER_PATTERNS_FILE:-}" ]]; then
  if [[ -f "${ENT_MARKER_PATTERNS_FILE}" ]]; then
    echo "Loading additional ENT marker patterns from: ${ENT_MARKER_PATTERNS_FILE}" >&2
    while IFS= read -r line; do
      [[ -z "$line" || "$line" == \#* ]] && continue
      patterns+=("$line")
    done < "${ENT_MARKER_PATTERNS_FILE}"
  else
    echo "::warning::ENT_MARKER_PATTERNS_FILE set but file not found: ${ENT_MARKER_PATTERNS_FILE}" >&2
  fi
fi

# ─────────────────────────────────────────────────────────────────────────────
# is_excluded <filepath>  → returns 0 (true) if the file should be skipped
# ─────────────────────────────────────────────────────────────────────────────
is_excluded() {
  local file="$1"
  for excl in "${EXCLUDE_PATHS[@]}"; do
    if [[ "$file" == *"$excl"* ]]; then
      return 0
    fi
  done
  return 1
}

# ─────────────────────────────────────────────────────────────────────────────
# Main scan
# ─────────────────────────────────────────────────────────────────────────────
violations=0
scanned=0

while IFS= read -r file; do
  # Skip blank lines
  [[ -z "$file" ]] && continue

  # Skip deleted or otherwise absent files
  [[ ! -f "$file" ]] && continue

  # Skip vendor / generated directories
  if [[ "$file" =~ $EXCLUDE_DIRS_PATTERN ]]; then
    continue
  fi

  # Only scan recognisable source / config file types
  if ! [[ "${file,,}" =~ $SCANNABLE_PATTERN ]]; then
    continue
  fi

  # Skip intentionally excluded paths
  if is_excluded "$file"; then
    echo "Skipping excluded file: ${file}" >&2
    continue
  fi

  scanned=$((scanned + 1))

  for pattern in "${patterns[@]}"; do
    if grep -qE "$pattern" "$file" 2>/dev/null; then
      # Capture the first matching line for the annotation message
      first_match=$(grep -nE "$pattern" "$file" 2>/dev/null | head -1 || true)
      echo "::error file=${file}::ENT content marker detected — pattern '${pattern}' → ${first_match}"
      violations=$((violations + 1))
    fi
  done

done < "$CHANGED_FILES_LIST"

echo "" >&2
echo "Files scanned : ${scanned}" >&2
echo "Violations    : ${violations}" >&2

if [[ "$violations" -gt 0 ]]; then
  echo "::error::ENT content marker check failed with ${violations} violation(s)." \
       "ENT-specific content must not be merged into a CE repository."
  exit 1
fi

echo "Content marker check passed: no ENT markers found in ${scanned} scanned file(s)."