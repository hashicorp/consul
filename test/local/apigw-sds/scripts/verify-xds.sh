#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

ADMIN_ADDR="${ADMIN_ADDR:-http://127.0.0.1:19000}"

DUMP_FILE="/tmp/envoy-config-dump.json"

curl -fsS "${ADMIN_ADDR}/config_dump" > "${DUMP_FILE}"

echo "Checking for expected SDS resources in listener config..."
if ! jq -e '..|objects|select(has("name") and .name=="foo.example.com")' "${DUMP_FILE}" >/dev/null; then
  echo "Did not find service override SDS resource name foo.example.com in config dump" >&2
  exit 1
fi

if ! jq -e '..|objects|select(has("name") and .name=="wildcard.ingress.consul")' "${DUMP_FILE}" >/dev/null; then
  echo "Did not find listener default SDS resource name wildcard.ingress.consul in config dump" >&2
  exit 1
fi

if ! jq -e '..|objects|select(has("cluster_name") and .cluster_name=="sds-cluster")' "${DUMP_FILE}" >/dev/null; then
  echo "Did not find sds-cluster reference in config dump" >&2
  exit 1
fi

echo "Checking filter-chain ordering (service SNI match before catch-all)..."
listener_json=$(curl -fsS "${ADMIN_ADDR}/config_dump?resource=dynamic_listeners")

first_has_sni=$(echo "${listener_json}" | jq -r '
  [..|objects|select(has("filter_chains"))|.filter_chains[]] as $chains
  | if ($chains|length) > 0 then (($chains[0].filter_chain_match.server_names // []) | length) > 0 else false end
')

if [[ "${first_has_sni}" != "true" ]]; then
  echo "First filter chain is not SNI-specific; expected service override chain first" >&2
  exit 1
fi

echo "xDS checks passed"

