#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1
#
# Triggers a consul (community edition) release promotion to staging or production via `bob`.
#
# Usage:
#   ./release-scripts/trigger-promotion.sh [staging|production] [options]
#
# The promotion target defaults to "staging" when no argument is provided.
#
# CE specifics: the CRT product version is a plain semantic version with no metadata
# suffix (e.g. 2.0.2) and the promotion targets the "consul" CRT environment (the
# hashicorp/consul -> consul mapping lives in crt-workflows-common's
# .github/env-policy.json).
#
# Options:
#   -n, --dry-run   Resolve all inputs and print the exact `bob` command WITHOUT
#                   executing it (also enabled by setting DRY_RUN=true).
#   -y, --yes       Non-interactive: skip the prompts/confirmation and use the
#                   values from the environment (or the derived defaults).
#   -h, --help      Show this help and exit.
#
# The version is prompted interactively with a [default]; press Enter to accept.
# It is seeded from CONSUL_RELEASE_VERSION when set, else the DEFAULT_* value below.
# REMOTE is an env-only override (default origin).
#
# Derived from the version (not prompted):
#   CONSUL_PRODUCT_VERSION = <CONSUL_RELEASE_VERSION>
#   CONSUL_RELEASE_BRANCH  = release/<CONSUL_RELEASE_VERSION>
#   CONSUL_RELEASE_SHA     = latest commit of <REMOTE>/<CONSUL_RELEASE_BRANCH>
# Setting CONSUL_PRODUCT_VERSION or CONSUL_RELEASE_BRANCH in the environment
# overrides the value derived from the version.

set -euo pipefail

usage() {
  sed -n '/^# Triggers /,/^set /p' "$0" | sed '/^set /d; s/^# \{0,1\}//; s/^#$//'
}

# -----------------------------------------------------------------------------
# Flags / arguments (promotion target plus dry-run / non-interactive switches)
# -----------------------------------------------------------------------------
DRY_RUN="${DRY_RUN:-false}"
INTERACTIVE=true
PROMOTION_TARGET=""
for arg in "$@"; do
  case "${arg}" in
    -n | --dry-run) DRY_RUN=true ;;
    -y | --yes) INTERACTIVE=false ;;
    -h | --help)
      usage
      exit 0
      ;;
    staging | production) PROMOTION_TARGET="${arg}" ;;
    *)
      echo "Unknown argument: ${arg}" >&2
      usage >&2
      exit 1
      ;;
  esac
done

# Normalize DRY_RUN to strictly "true" or "false" (accepts true/1/yes from env).
case "${DRY_RUN}" in
  true | TRUE | True | 1 | yes | YES) DRY_RUN=true ;;
  *) DRY_RUN=false ;;
esac

PROMOTION_TARGET="${PROMOTION_TARGET:-staging}"

# run CMD...  Runs CMD, or in dry-run mode prints it (shell-quoted) instead.
run() {
  if [[ "${DRY_RUN}" == "true" ]]; then
    { printf '  [dry-run] $'; printf ' %q' "$@"; printf '\n'; }
  else
    "$@"
  fi
}

# fail_or_warn MESSAGE  Errors out normally; in dry-run only warns so the preview
# can run to completion.
fail_or_warn() {
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "Warning (dry-run): $1" >&2
  else
    echo "Error: $1" >&2
    exit 1
  fi
}

# prompt_var VAR_NAME DEFAULT_VALUE [required]
# Shows the value that will be used and lets the user accept or override it.
prompt_var() {
  local var_name="$1"
  local default_value="$2"
  local required="${3:-}"
  local input
  while :; do
    read -r -p "  ${var_name} [${default_value}]: " input || true
    input="${input:-${default_value}}"
    if [[ -z "${input}" && "${required}" == "required" ]]; then
      echo "  ${var_name} is required; please enter a value." >&2
      continue
    fi
    break
  done
  printf -v "${var_name}" '%s' "${input}"
}

REMOTE="${REMOTE:-origin}"

# -----------------------------------------------------------------------------
# Defaults offered at the prompt. Edit this to change the default.
# -----------------------------------------------------------------------------
DEFAULT_CONSUL_RELEASE_VERSION=""

# -----------------------------------------------------------------------------
# Collect the version to promote (seeded from the environment or the DEFAULT_*
# value above; the prompt is shown only in interactive mode). The promotion
# target is the positional argument resolved above.
# -----------------------------------------------------------------------------
CONSUL_RELEASE_VERSION="${CONSUL_RELEASE_VERSION:-${DEFAULT_CONSUL_RELEASE_VERSION}}"
if [[ "${INTERACTIVE}" == "true" ]]; then
  echo "Enter the version to promote (press Enter to accept the [default]):"
  echo
  prompt_var CONSUL_RELEASE_VERSION "${CONSUL_RELEASE_VERSION}" required
  echo
fi
: "${CONSUL_RELEASE_VERSION:?CONSUL_RELEASE_VERSION is required (set it or run interactively)}"
# Normalize: strip any leading "v" so we compose consistently.
CONSUL_RELEASE_VERSION="${CONSUL_RELEASE_VERSION#v}"

# Validate the version (bad input is fatal even in dry-run). A prerelease suffix
# (e.g. 1.22.9-rc1) is allowed for release-candidate promotions.
if [[ ! "${CONSUL_RELEASE_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "Error: CONSUL_RELEASE_VERSION must be MAJOR.MINOR.PATCH (e.g. 1.22.9), got '${CONSUL_RELEASE_VERSION}'." >&2
  exit 1
fi

# Derive everything else from the version. The release branch is release/<version>.
# Both accept an env override for unusual cases.
CONSUL_PRODUCT_VERSION="${CONSUL_PRODUCT_VERSION:-${CONSUL_RELEASE_VERSION}}"
CONSUL_RELEASE_BRANCH="${CONSUL_RELEASE_BRANCH:-release/${CONSUL_RELEASE_VERSION}}"
# Ensure the product version is clean.
CONSUL_PRODUCT_VERSION="${CONSUL_PRODUCT_VERSION#v}"

# Export so child processes (e.g. bob) inherit the resolved values.
export CONSUL_RELEASE_VERSION CONSUL_PRODUCT_VERSION CONSUL_RELEASE_BRANCH REMOTE

# -----------------------------------------------------------------------------
# Prerequisite checks (git is always required; bob only for a real promotion)
# -----------------------------------------------------------------------------
if ! command -v git >/dev/null 2>&1; then
  echo "Error: required command 'git' not found in PATH." >&2
  exit 1
fi
if ! command -v bob >/dev/null 2>&1; then
  fail_or_warn "required command 'bob' not found in PATH."
fi

# Ensure the remote ref is current so we resolve the latest commit SHA. This is a
# read-only operation, so it runs even in dry-run to print an accurate command.
echo "Fetching latest refs for ${CONSUL_RELEASE_BRANCH} from ${REMOTE}..."
if ! git fetch "${REMOTE}" "${CONSUL_RELEASE_BRANCH}"; then
  fail_or_warn "'git fetch' failed for ${CONSUL_RELEASE_BRANCH}."
fi

# Resolve the latest commit SHA of the release branch.
if ! CONSUL_RELEASE_SHA="$(git rev-parse "${REMOTE}/${CONSUL_RELEASE_BRANCH}" 2>/dev/null)"; then
  fail_or_warn "unable to resolve ${REMOTE}/${CONSUL_RELEASE_BRANCH}. Does the branch exist on ${REMOTE}?"
  CONSUL_RELEASE_SHA="<unresolved-sha-for-${REMOTE}/${CONSUL_RELEASE_BRANCH}>"
fi
export CONSUL_RELEASE_SHA

# -----------------------------------------------------------------------------
# Print configuration and confirm
# -----------------------------------------------------------------------------
cat <<EOF

The following variables are set:

  CONSUL_RELEASE_VERSION               = ${CONSUL_RELEASE_VERSION}
  CONSUL_PRODUCT_VERSION               = ${CONSUL_PRODUCT_VERSION}
  CONSUL_RELEASE_BRANCH                = ${CONSUL_RELEASE_BRANCH}
  CONSUL_RELEASE_SHA                   = ${CONSUL_RELEASE_SHA}
  REMOTE                               = ${REMOTE}

  Promotion target                     = ${PROMOTION_TARGET}
  Dry run                              = ${DRY_RUN}

EOF

# -----------------------------------------------------------------------------
# Build the promotion command (single source of truth for run + dry-run)
# -----------------------------------------------------------------------------
promotion_cmd=(
  bob trigger-promotion
  --product-name=consul
  --repo=consul
  --product-version="${CONSUL_PRODUCT_VERSION}"
  --sha="${CONSUL_RELEASE_SHA}"
  --environment=consul-oss
  --slack-channel=C09KX8B2KC6
  --org=hashicorp
  --branch="${CONSUL_RELEASE_BRANCH}"
  "${PROMOTION_TARGET}"
)

if [[ "${DRY_RUN}" == "true" ]]; then
  echo ">>> DRY RUN: no promotion will be triggered."
  echo ">>> The command that would run is printed below, prefixed with [dry-run]."
  echo
elif [[ "${INTERACTIVE}" == "true" ]]; then
  read -r -p "Proceed with promotion to ${PROMOTION_TARGET}? [y/N] " response || true
  case "${response}" in
    [yY] | [yY][eE][sS]) ;;
    *)
      echo "Aborted. No promotion was triggered."
      exit 1
      ;;
  esac
fi

# -----------------------------------------------------------------------------
# Trigger the promotion (printed in dry-run, executed otherwise)
# -----------------------------------------------------------------------------
echo "==> Triggering promotion to ${PROMOTION_TARGET}..."
run "${promotion_cmd[@]}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo
  echo "Dry run complete. No promotion was triggered."
fi
