#!/usr/bin/env bash
set -euo pipefail

ADMIN_ADDR="${ADMIN_ADDR:-http://127.0.0.1:29000}"
DUMP_FILE="/tmp/ingress-envoy-config-dump.json"

curl -fsS "${ADMIN_ADDR}/config_dump" > "${DUMP_FILE}"

echo "Checking for expected SDS resources in ingress config dump..."
if ! jq -e '..|objects|select(has("name") and .name=="foo.example.com")' "${DUMP_FILE}" >/dev/null; then
  echo "Did not find service SDS resource name foo.example.com in config dump" >&2
  exit 1
fi

if ! jq -e '..|objects|select(has("cluster_name") and .cluster_name=="sds-cluster")' "${DUMP_FILE}" >/dev/null; then
  echo "Did not find sds-cluster reference in config dump" >&2
  exit 1
fi

echo "Checking SDS stats endpoint..."
curl -fsS "${ADMIN_ADDR}/stats?filter=sds" | sed -n '1,80p'

echo "xDS checks passed"

