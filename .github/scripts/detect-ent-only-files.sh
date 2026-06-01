#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# detect-ent-only-files.sh
#
# Dynamically detects files present in an ENT (enterprise) repository that are
# absent from the paired CE (community edition) repository. The comparison is
# fully automated — no manually maintained allow/deny lists are required, so
# the check automatically adapts as both repos evolve.
#
# Usage:
#   ./detect-ent-only-files.sh \
#       --ce-repo  <owner/repo> \
#       --ent-repo <owner/repo> \
#       [--ce-ref  <branch-or-sha>] \
#       [--ent-ref <branch-or-sha>]
#
# Output:
#   Newline-separated list of file paths that exist only in the ENT repo
#   (written to stdout). Empty output means no ENT-exclusive files were found.
#
# Exit codes:
#   0  Success (list written to stdout; may be empty)
#   1  Fatal error (missing args, API failure, truncated tree with no fallback)
#
# Requirements:
#   - gh CLI installed and authenticated (GH_TOKEN env var must be set)
#     The token needs read:contents scope on the ENT repo only.
#     CE file listing is performed via local git (no token required).
#
# Extending to another product pair (e.g., consul / consul-enterprise):
#   Pass different --ce-repo and --ent-repo values. The script is stateless
#   and product-agnostic.

set -euo pipefail

CE_REPO=""
ENT_REPO=""
CE_REF="main"
ENT_REF="main"

usage() {
  echo "Usage: $0 --ce-repo <owner/repo> --ent-repo <owner/repo> [--ce-ref <branch>] [--ent-ref <branch>]" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ce-repo)  CE_REPO="$2";  shift 2 ;;
    --ent-repo) ENT_REPO="$2"; shift 2 ;;
    --ce-ref)   CE_REF="$2";   shift 2 ;;
    --ent-ref)  ENT_REF="$2";  shift 2 ;;
    *) echo "Unknown argument: $1" >&2; usage ;;
  esac
done

[[ -z "$CE_REPO" ]]  && { echo "Error: --ce-repo is required"  >&2; usage; }
[[ -z "$ENT_REPO" ]] && { echo "Error: --ent-repo is required" >&2; usage; }

# Signal file used to communicate a graceful 404-skip from fetch_file_tree
# back to the parent scope.  fetch_file_tree runs in a subshell (via $()),
# so it cannot set parent-shell variables directly.
_ENT_SKIP_SIGNAL=$(mktemp)

# ─────────────────────────────────────────────────────────────────────────────
# fetch_file_tree <owner/repo> <ref>
#
# Fetches the full recursive Git blob tree for a repository at a given ref via
# the GitHub API, printing one file path per line to stdout.
#
# Large repositories (>100 000 objects) may return a truncated tree. When that
# happens we fall back to listing files directory by directory using the
# contents API (slower but complete). If both approaches fail, the function
# exits with an error so the calling workflow fails closed (secure default).
# ─────────────────────────────────────────────────────────────────────────────
fetch_file_tree() {
  local repo="$1"
  local ref="$2"
  local response

  echo "Fetching file tree for ${repo}@${ref} ..." >&2

  response=$(gh api "repos/${repo}/git/trees/${ref}?recursive=1" 2>&1) || {
    # Distinguish between "repo not found / no token access" (404) and other
    # failures.  A 404 most commonly means the token (GITHUB_TOKEN) does not
    # have cross-org read access to the ENT repository.  In that case we emit
    # a warning and return an empty file list so the workflow does not block
    # legitimate CE-only PRs.  Set the ENT_READ_TOKEN secret to a PAT with
    # read:contents access on the ENT repo to enable the full comparison.
    if echo "${response}" | grep -q '"status":"404"\|HTTP 404\|Not Found'; then
      echo "::warning::Could not fetch file tree for ${repo}@${ref} (HTTP 404)." \
           "The token in use likely lacks cross-org read access to ${repo}." \
           "Ensure the GH_TOKEN_TEMP repository secret has read:contents scope" \
           "on ${repo} to enable ENT-exclusive file detection." \
           "Skipping ENT-file comparison for this run." >&2
      echo "skipped" > "${_ENT_SKIP_SIGNAL}"
      return 0
    fi
    echo "::error::Failed to fetch file tree for ${repo}@${ref}: ${response}" >&2
    exit 1
  }

  local truncated
  truncated=$(echo "$response" | jq -r '.truncated')

  if [[ "$truncated" == "true" ]]; then
    echo "::warning::File tree for ${repo}@${ref} is truncated (repo likely has >100k objects)." \
         "Falling back to the Search API — this may be slower." >&2
    # Fallback: use the GitHub Search API to list all files in the repo.
    # This is rate-limited (30 requests/min for authenticated users) so we warn
    # but still proceed rather than silently returning an incomplete list.
    gh api \
      --paginate \
      "search/code?q=repo:${repo}&per_page=100" \
      --jq '.items[].path' 2>/dev/null \
      | sort -u \
      || {
        echo "::error::Fallback file listing for ${repo} also failed. Cannot guarantee a complete ENT-file list." >&2
        exit 1
      }
    return
  fi

  echo "$response" | jq -r '.tree[] | select(.type == "blob") | .path'
}

# ─────────────────────────────────────────────────────────────────────────────
# CE file list — derived from the local git checkout (no API call / token
# required).  The CE repo is always checked out by the calling workflow, so
# git ls-tree against the remote tracking ref is reliable and fast.
# ─────────────────────────────────────────────────────────────────────────────
echo "Listing CE file tree for ${CE_REPO}@${CE_REF} from local git..." >&2
ce_files=$(git ls-tree -r --name-only "origin/${CE_REF}" 2>/dev/null) || {
  # Fallback: try the ref without the origin/ prefix (e.g. a bare SHA or tag).
  ce_files=$(git ls-tree -r --name-only "${CE_REF}" 2>/dev/null) || {
    echo "::error::Could not list CE file tree for ${CE_REPO}@${CE_REF} from local git." >&2
    exit 1
  }
}
echo "CE  file count : $(echo "$ce_files" | grep -c . || true)" >&2

# ENT file list — fetched from the GitHub API using GH_TOKEN.
ent_files=$(fetch_file_tree "$ENT_REPO" "$ENT_REF")

# If fetch_file_tree signalled a graceful 404-skip, exit cleanly.
# The warning was already emitted inside the function.
if [[ -s "${_ENT_SKIP_SIGNAL}" ]]; then
  rm -f "${_ENT_SKIP_SIGNAL}"
  echo "ENT file check skipped: ENT repository not accessible with the current token." >&2
  exit 0
fi
rm -f "${_ENT_SKIP_SIGNAL}"

if [[ -z "$ent_files" ]]; then
  echo "::error::ENT file tree for ${ENT_REPO}@${ENT_REF} is empty — cannot determine ENT-exclusive files." >&2
  exit 1
fi

echo "ENT file count : $(echo "$ent_files" | grep -c . || true)" >&2

# Output lines that appear in ENT but NOT in CE (ENT-exclusive paths)
comm -23 \
  <(echo "$ent_files" | sort) \
  <(echo "$ce_files"  | sort)
