#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0
#
# Verifies that the released consul Docker image is available
# in the HashiCorp registry and reports its version and image digest (SHA).
#
# Workflow:
#   1. Pull  hashicorp/consul:<version>  (prints pull output)
#   2. Print the image digest (sha256) and image ID
#   3. Run   `consul version` inside the image (prints output)
#
# Usage:
#   ./verify-release-image.sh [staging|production]
#   CONSUL_PRODUCT_VERSION=2.0.2 ./verify-release-image.sh staging
#
# If no environment argument is provided, it defaults to production.
# The version is prompted interactively with a [default]; press Enter to accept.
# Setting CONSUL_PRODUCT_VERSION in the environment pre-fills the default.

set -euo pipefail

# -----------------------------------------------------------------------------
# Defaults offered at the prompt. Edit these to change the defaults.
# -----------------------------------------------------------------------------
DEFAULT_CONSUL_PRODUCT_VERSION=2.0.2

# -----------------------------------------------------------------------------
# Arguments
# -----------------------------------------------------------------------------
ENVIRONMENT=""
for arg in "$@"; do
  case "${arg}" in
    staging | production) ENVIRONMENT="${arg}" ;;
    -h | --help)
      echo "Usage: ./verify-release-image.sh [staging|production]"
      exit 0
      ;;
    *)
      echo "Unknown argument: ${arg}" >&2
      exit 1
      ;;
  esac
done

ENVIRONMENT="${ENVIRONMENT:-production}"

if [[ "${ENVIRONMENT}" == "staging" ]]; then
  IMAGE_REPO="${IMAGE_REPO:-crt-core-staging-docker-local.artifactory.hashicorp.engineering/docker.io/hashicorp/consul}"
else
  IMAGE_REPO="${IMAGE_REPO:-hashicorp/consul}"
fi

# prompt_var VAR_NAME DEFAULT_VALUE
# Prompts for a value, showing the default; an empty reply keeps the default.
prompt_var() {
  local var_name="$1"
  local default_value="$2"
  local input
  read -r -p "  ${var_name} [${default_value}]: " input || true
  printf -v "${var_name}" '%s' "${input:-${default_value}}"
  export "${var_name}"
}

# -----------------------------------------------------------------------------
# Prerequisite checks
# -----------------------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
  echo "Error: required command 'docker' not found in PATH." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Error: the Docker daemon is not running or not reachable." >&2
  exit 1
fi

# -----------------------------------------------------------------------------
# Collect inputs (an existing env var value pre-fills the default)
# -----------------------------------------------------------------------------
echo "Enter the version to verify (press Enter to accept the [default]):"
prompt_var CONSUL_PRODUCT_VERSION \
  "${CONSUL_PRODUCT_VERSION:-${DEFAULT_CONSUL_PRODUCT_VERSION}}"
echo

IMAGE_REF="${IMAGE_REPO}:${CONSUL_PRODUCT_VERSION}"

# -----------------------------------------------------------------------------
# 1. Pull the image (docker pull prints its own progress and final digest)
# -----------------------------------------------------------------------------
echo "==> docker pull ${IMAGE_REF}"
echo
if ! docker pull "${IMAGE_REF}"; then
  echo >&2
  echo "Error: failed to pull ${IMAGE_REF}. Is this version published in the registry?" >&2
  exit 1
fi
echo

# -----------------------------------------------------------------------------
# 2. Report the image digest (SHA) and image ID
# -----------------------------------------------------------------------------
echo "==> Image digest (SHA) for ${IMAGE_REF}:"
DIGEST="$(docker inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "${IMAGE_REF}" 2>/dev/null || true)"
if [[ -n "${DIGEST}" ]]; then
  printf '%s' "${DIGEST}"
else
  echo "  (no RepoDigests found for ${IMAGE_REF})"
fi

IMAGE_ID="$(docker inspect --format '{{.Id}}' "${IMAGE_REF}" 2>/dev/null || true)"
echo "  Image ID: ${IMAGE_ID}"
echo

# -----------------------------------------------------------------------------
# 3. Print the consul version from inside the image
# -----------------------------------------------------------------------------
# Only allocate a TTY (-t) when stdout is a terminal so this also works in CI,
# where attaching a TTY would fail with "the input device is not a TTY".
run_flags=(--rm -i)
if [[ -t 1 ]]; then
  run_flags+=(-t)
fi

echo "==> docker run --rm -ti ${IMAGE_REF} version"
echo
docker run "${run_flags[@]}" "${IMAGE_REF}" version
echo

echo "==> Verification complete for ${IMAGE_REF}."
