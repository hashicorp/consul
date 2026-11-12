#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Verifies that the Envoy version bundled in consul-dataplane is within the range of
# Envoy versions this branch of Consul supports.
#
# consul-dataplane ships an Envoy binary inside its image, and consul-k8s runs that
# binary rather than one the operator picks. If the bundled version drifts above the
# newest minor in ENVOY_VERSIONS, `consul connect envoy` refuses to bootstrap the proxy,
# because the constraint in command/connect/envoy/envoy.go is built from
# xdscommon.GetMaxEnvoyMajorVersion(). The two repositories therefore have to move
# together, and this check makes the drift visible from the Consul side.
#
# Usage: verify_dataplane_envoy_version.sh [dataplane-ref]

DATAPLANE_REF="${1:-${DATAPLANE_REF:-main}}"
DATAPLANE_REPO="${DATAPLANE_REPO:-hashicorp/consul-dataplane}"
ENVOY_VERSIONS_FILE="${ENVOY_VERSIONS_FILE:-envoyextensions/xdscommon/ENVOY_VERSIONS}"

if [ ! -f "$ENVOY_VERSIONS_FILE" ]; then
  echo "ERROR! $ENVOY_VERSIONS_FILE not found. Run this script from the repository root."
  exit 1
fi

# minor_version strips the patch number, e.g. 1.38.4 => 1.38
minor_version() {
  cut -d "." -f1-2 <<<"$1"
}

# version_le returns success when $1 sorts at or before $2.
version_le() {
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n 1)" = "$1" ]
}

supported_versions=$(grep '^[[:digit:]]' "$ENVOY_VERSIONS_FILE" | sort -Vr)
if [ -z "$supported_versions" ]; then
  echo "ERROR! No Envoy versions found in $ENVOY_VERSIONS_FILE"
  exit 1
fi

max_supported_minor=$(minor_version "$(head -n 1 <<<"$supported_versions")")
min_supported_minor=$(minor_version "$(tail -n 1 <<<"$supported_versions")")

dockerfile_url="https://raw.githubusercontent.com/${DATAPLANE_REPO}/${DATAPLANE_REF}/Dockerfile"
dockerfile=$(curl -fsSL "$dockerfile_url") || {
  echo "ERROR! Could not fetch $dockerfile_url"
  exit 1
}

dataplane_envoy_version=$(grep -m 1 '^ARG ENVOY_VERSION=' <<<"$dockerfile" | cut -d "=" -f2)
if ! [[ $dataplane_envoy_version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "ERROR! Could not read ARG ENVOY_VERSION from $dockerfile_url, got: '$dataplane_envoy_version'"
  exit 1
fi
dataplane_envoy_minor=$(minor_version "$dataplane_envoy_version")

echo "Consul branch supports Envoy ${min_supported_minor}.x through ${max_supported_minor}.x"
echo "${DATAPLANE_REPO}@${DATAPLANE_REF} bundles Envoy ${dataplane_envoy_version}"

if ! version_le "$min_supported_minor" "$dataplane_envoy_minor"; then
  echo
  echo "ERROR! Envoy ${dataplane_envoy_version} is older than the minimum supported Envoy ${min_supported_minor}.x."
  echo "Sidecars using this consul-dataplane image will be rejected as too old by the xDS server."
  exit 1
fi

if ! version_le "$dataplane_envoy_minor" "$max_supported_minor"; then
  echo
  echo "ERROR! Envoy ${dataplane_envoy_version} is newer than the maximum supported Envoy ${max_supported_minor}.x."
  echo "'consul connect envoy' will refuse to bootstrap proxies using this consul-dataplane image."
  echo "Update ${ENVOY_VERSIONS_FILE} on this branch, or pin an older Envoy in consul-dataplane."
  exit 1
fi

echo "#### SUCCESS! #### consul-dataplane Envoy ${dataplane_envoy_version} is within the supported range."
