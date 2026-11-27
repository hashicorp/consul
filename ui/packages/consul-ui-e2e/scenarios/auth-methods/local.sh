#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# Local-binary runner for `auth-methods` — the ONLY supported runtime for
# this scenario (see scenario.json's requiresLocalRuntime and README.md for
# why: Consul, the browser, and the Dex IdP all need to agree on one
# hostname for OIDC's issuer/redirect-URI matching, which breaks across the
# Docker container/host boundary).
#
# Unlike the plain template, this DOES still use Docker — just only for the
# Dex sidecar (`docker-compose.yml`, no `consul` service in it). Consul
# itself runs as a local binary alongside it.
#
# Needs a Consul Enterprise binary + license (the seeded auth methods carry
# Enterprise-only NamespaceRules, and the UI only offers SSO login on
# Enterprise): set CONSUL_BIN and CONSUL_LICENSE_PATH.
#
#   ./local.sh up     # start Dex (docker) + a Consul dev agent (local), on
#                      # their fixed ports; seeding is done by seed.ts
#   ./local.sh down   # stop both
set -euo pipefail

CONSUL_BIN="${CONSUL_BIN:-}"
LICENSE_PATH="${CONSUL_LICENSE_PATH:-}"
DIR="$(cd "$(dirname "$0")" && pwd)"
RUN_DIR="${RUN_DIR:-$DIR/.run}"
PID_FILE="$RUN_DIR/agent.pid"
LOG_FILE="$RUN_DIR/agent.log"
COMPOSE_PROJECT="consul-e2e-auth-methods-dex"
DEX_URL="http://127.0.0.1:5556/dex/.well-known/openid-configuration"

wait_for() {
  local label="$1" url="$2" tries=30
  for ((i = 0; i < tries; i++)); do
    curl -sf "$url" >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "$label did not come up in time (checked $url)"; exit 1
}

up() {
  mkdir -p "$RUN_DIR"
  [[ -n "$CONSUL_BIN" ]] || { echo "CONSUL_BIN not set — point it at a Consul Enterprise binary"; exit 1; }
  command -v "$CONSUL_BIN" >/dev/null || { echo "consul binary not found at CONSUL_BIN=$CONSUL_BIN"; exit 1; }
  [[ -n "$LICENSE_PATH" ]] || { echo "CONSUL_LICENSE_PATH not set — point it at your .hclic license file"; exit 1; }
  [[ -f "$LICENSE_PATH" ]] || { echo "license not found at CONSUL_LICENSE_PATH=$LICENSE_PATH"; exit 1; }
  command -v docker >/dev/null || { echo "docker not found — needed to run the Dex IdP sidecar"; exit 1; }
  down || true

  echo "starting Dex (docker) on :5556 ..."
  (cd "$DIR" && docker compose -p "$COMPOSE_PROJECT" up -d)
  wait_for "Dex" "$DEX_URL"

  echo "starting Consul dev agent (local) on :8500 ..."
  # ACLs enabled with a fixed initial_management token — same fixture
  # pattern as scenarios/namespaces, doubles as the UI login token. Bound to
  # 127.0.0.1 for the same reason as Dex's issuer: consistency with what the
  # browser and seed.ts's OIDC auth-method Config both use — see README.md.
  CONSUL_LICENSE_PATH="$LICENSE_PATH" "$CONSUL_BIN" agent -dev -node node-primary -bind 127.0.0.1 -client 127.0.0.1 -dns-port=-1 \
    -hcl 'acl { enabled = true default_policy = "allow" tokens { initial_management = "1234" } }' \
    > "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  echo "consul dev agent started (PID $(cat "$PID_FILE")) — logs: $LOG_FILE"
}

down() {
  if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    local pid; pid="$(cat "$PID_FILE")"
    kill "$pid"
    # `up` runs `down` first, then immediately rebinds :8500 — without
    # actually waiting for SIGTERM to take effect, the old process can still
    # be holding the port when the new one tries to start, so the new PID
    # gets logged but the OLD agent (with its own already-seeded state) is
    # who actually answers requests. Wait for the exit for real.
    for ((i = 0; i < 20; i++)); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -0 "$pid" 2>/dev/null && { echo "agent $pid did not exit in time"; kill -9 "$pid" 2>/dev/null || true; }
    echo "stopped agent $pid"
  fi
  rm -f "$PID_FILE"
  (cd "$DIR" && docker compose -p "$COMPOSE_PROJECT" down -v) 2>/dev/null || true
}

case "${1:-up}" in
  up) up ;;
  down) down ;;
  *) echo "usage: $0 {up|down}"; exit 1 ;;
esac
