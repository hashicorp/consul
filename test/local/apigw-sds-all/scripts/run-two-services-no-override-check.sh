#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONSUL_HTTP_ADDR="${CONSUL_HTTP_ADDR:-http://127.0.0.1:19700}"
GATEWAY_HOST="${GATEWAY_HOST:-127.0.0.1}"
GATEWAY_HTTPS_PORT="${GATEWAY_HTTPS_PORT:-29643}"
ADMIN_ADDR="${ADMIN_ADDR:-http://127.0.0.1:39200}"
ENVOY_IMAGE="${ENVOY_IMAGE:-envoyproxy/envoy:v1.34.12}"

REQUEST_HOSTNAME="${REQUEST_HOSTNAME:-bar.example.com}"
SECOND_HOSTNAME="${SECOND_HOSTNAME:-www.example.com}"
EXPECTED_CERT_CN="${EXPECTED_CERT_CN:-*.ingress.consul}"
EXPECTED_CERT_SAN="${EXPECTED_CERT_SAN:-*.ingress.consul}"
EXPECTED_ISSUER_CN="${EXPECTED_ISSUER_CN:-SDS Test CA Cert}"

cd "${ROOT_DIR}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

for c in docker consul curl jq openssl python3; do
  require_cmd "$c"
done

send_https() {
  local host="$1"
  printf 'GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n' "${host}" | \
    openssl s_client -quiet -connect "${GATEWAY_HOST}:${GATEWAY_HTTPS_PORT}" -servername "${host}" 2>/dev/null
}

wait_bound() {
  local kind="$1"
  local name="$2"
  local i json

  for i in $(seq 1 40); do
    json="$(consul config read -http-addr="${CONSUL_HTTP_ADDR}" -kind "${kind}" -name "${name}" 2>/dev/null || true)"
    if [ -n "${json}" ] && JSON_INPUT="${json}" python3 -u - <<'PY'
import json
import os
import sys
obj = json.loads(os.environ['JSON_INPUT'])
conds = obj.get('Status', {}).get('Conditions', [])
state = {(c.get('Type'), c.get('Status')) for c in conds}
sys.exit(0 if ('Accepted', 'True') in state and ('Bound', 'True') in state else 1)
PY
    then
      return 0
    fi
    sleep 1
  done

  echo "FAIL: ${kind}/${name} did not reach Accepted=True and Bound=True" >&2
  exit 1
}

wait_http_contains() {
  local host="$1"
  local expected="$2"
  local label="$3"
  local i out

  for i in $(seq 1 30); do
    out="$(send_https "${host}" || true)"
    if echo "${out}" | grep -q "${expected}"; then
      echo "PASS: ${label}"
      echo "${out}" | sed -n '1,20p'
      return 0
    fi
    sleep 1
  done

  echo "FAIL: ${label}; expected response containing ${expected}" >&2
  echo "${out:-}" | sed -n '1,20p' >&2
  exit 1
}

extract_cert_identity() {
  local host="$1"
  openssl s_client -connect "${GATEWAY_HOST}:${GATEWAY_HTTPS_PORT}" -servername "${host}" -showcerts < /dev/null 2>/dev/null | \
    openssl x509 -noout -subject -issuer -ext subjectAltName
}

assert_cert_identity() {
  local host="$1"
  local expected_cn="$2"
  local expected_san="$3"
  local expected_issuer_cn="$4"

  local identity principal
  identity="$(extract_cert_identity "${host}")"

  if ! echo "${identity}" | grep -Fq "subject=" || ! echo "${identity}" | grep -Fq "CN=${expected_cn}"; then
    echo "FAIL: SNI ${host} expected subject CN=${expected_cn}" >&2
    echo "${identity}" >&2
    exit 1
  fi
  if ! echo "${identity}" | grep -Fq "DNS:${expected_san}"; then
    echo "FAIL: SNI ${host} expected SAN DNS:${expected_san}" >&2
    echo "${identity}" >&2
    exit 1
  fi
  if ! echo "${identity}" | grep -Fq "issuer=" || ! echo "${identity}" | grep -Fq "CN=${expected_issuer_cn}"; then
    echo "FAIL: SNI ${host} expected issuer CN=${expected_issuer_cn}" >&2
    echo "${identity}" >&2
    exit 1
  fi

  principal="$(echo "${identity}" | awk -F'DNS:' '/DNS:/{print $2}' | awk -F',' '{print $1}' | tr -d '[:space:]')"

  echo "PASS: cert identity for ${host}"
  echo "${identity}"
  echo "principal(SAN DNS): ${principal}"
}

listener_sds_refs() {
  curl -fsS "${ADMIN_ADDR}/config_dump" | jq -r '.. | objects | select(has("tls_certificate_sds_secret_configs")) | .tls_certificate_sds_secret_configs[]?.name' | sort -u
}

active_secret_names() {
  curl -fsS "${ADMIN_ADDR}/config_dump" | jq -r '.. | objects | select(has("dynamic_active_secrets")) | .dynamic_active_secrets[]?.name' | sort -u
}

assert_sds_state() {
  local refs secrets
  refs="$(listener_sds_refs || true)"
  secrets="$(active_secret_names || true)"

  echo "--- listener SDS refs ---"
  echo "${refs}"
  echo "--- active dynamic secrets ---"
  echo "${secrets}"

  if ! echo "${refs}" | grep -Fxq 'wildcard.ingress.consul'; then
    echo "FAIL: expected listener SDS ref wildcard.ingress.consul" >&2
    exit 1
  fi
  if echo "${refs}" | grep -Fxq 'foo.example.com'; then
    echo "FAIL: unexpected override SDS ref foo.example.com in no-override scenario" >&2
    exit 1
  fi
  if ! echo "${secrets}" | grep -Fxq 'wildcard.ingress.consul'; then
    echo "FAIL: expected active secret wildcard.ingress.consul" >&2
    exit 1
  fi
}

cleanup_routes() {
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route-mixed-override >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route-default-only >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route-two-services-no-override >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route-no-override-a >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind http-route -name http-route-no-override-b >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind tcp-route -name tcp-route >/dev/null 2>&1 || true
  consul config delete -http-addr="${CONSUL_HTTP_ADDR}" -kind tcp-route -name tcp-route-conflict >/dev/null 2>&1 || true
}

trap cleanup_routes EXIT

echo "[1/9] docker compose up"
docker compose up -d

echo "[2/9] bootstrap"
bash ./scripts/bootstrap.sh

echo "[3/9] launch api gateway envoy with ${ENVOY_IMAGE}"
ENVOY_IMAGE="${ENVOY_IMAGE}" bash ./scripts/launch-envoy.sh

echo "[4/9] launch sidecars"
SERVICE_NAME=svc-http SIDECAR_CONTAINER=svc-http-sidecar-all SIDECAR_ADMIN_PORT=39201 bash ./scripts/launch-sidecar.sh
SERVICE_NAME=svc-http-2 SIDECAR_CONTAINER=svc-http2-sidecar-all SIDECAR_ADMIN_PORT=39203 bash ./scripts/launch-sidecar.sh

echo "[5/9] service-defaults for svc-http-2"
consul config write -http-addr="${CONSUL_HTTP_ADDR}" - <<'HCL'
Kind = "service-defaults"
Name = "svc-http-2"
Protocol = "http"
HCL

echo "[6/9] start backends"
docker rm -f svc-http-backend-all >/dev/null 2>&1 || true
docker run -d --name svc-http-backend-all --network container:svc-http-sidecar-all \
  hashicorp/http-echo:1.0.0 -text hello-http-1 -listen :5678 >/dev/null

docker rm -f svc-http2-backend-all >/dev/null 2>&1 || true
docker run -d --name svc-http2-backend-all --network container:svc-http2-sidecar-all \
  hashicorp/http-echo:1.0.0 -text hello-http-2 -listen :5678 >/dev/null

echo "[7/9] write two no-override routes (one per service)"
cleanup_routes
consul config write -http-addr="${CONSUL_HTTP_ADDR}" - <<'HCL'
Kind = "http-route"
Name = "http-route-no-override-a"

Parents = [{
  Kind = "api-gateway"
  Name = "api-gw"
  SectionName = "https"
}]

Hostnames = ["bar.example.com"]

Rules = [{
  Matches = [{
    Path = {
      Match = "prefix"
      Value = "/"
    }
  }]
  Services = [{ Name = "svc-http" }]
}]
HCL

consul config write -http-addr="${CONSUL_HTTP_ADDR}" - <<'HCL'
Kind = "http-route"
Name = "http-route-no-override-b"

Parents = [{
  Kind = "api-gateway"
  Name = "api-gw"
  SectionName = "https"
}]

Hostnames = ["www.example.com"]

Rules = [{
  Matches = [{
    Path = {
      Match = "prefix"
      Value = "/"
    }
  }]
  Services = [{ Name = "svc-http-2" }]
}]
HCL

echo "[8/9] wait for Accepted=True and Bound=True"
wait_bound http-route http-route-no-override-a
wait_bound http-route http-route-no-override-b

echo "[9/9] verify traffic, cert, and SDS refs"
wait_http_contains "${REQUEST_HOSTNAME}" "hello-http-1" "bar host routes to svc-http"
wait_http_contains "${SECOND_HOSTNAME}" "hello-http-2" "www host routes to svc-http-2"
assert_cert_identity "${REQUEST_HOSTNAME}" "${EXPECTED_CERT_CN}" "${EXPECTED_CERT_SAN}" "${EXPECTED_ISSUER_CN}"
assert_cert_identity "${SECOND_HOSTNAME}" "${EXPECTED_CERT_CN}" "${EXPECTED_CERT_SAN}" "${EXPECTED_ISSUER_CN}"
assert_sds_state

echo "PASS: no-override two-service behavior verified"

