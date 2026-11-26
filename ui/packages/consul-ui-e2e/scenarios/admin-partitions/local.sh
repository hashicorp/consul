#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# Local-binary fallback runner (RUNTIME=local) for the Enterprise
# `admin-partitions` scenario. Unlike the plain template, this genuinely
# needs an Enterprise binary + a license — set CONSUL_BIN and
# CONSUL_LICENSE_PATH (see ../../.env.example) before running.
#
#   CONSUL_BIN=/path/to/ent/consul CONSUL_LICENSE_PATH=/path/to.hclic ./local.sh up
#   ./local.sh down
#
# The scenario CLI runs `seed.ts` separately, so this only manages the agent.
set -euo pipefail

CONSUL_BIN="${CONSUL_BIN:-}"
LICENSE_PATH="${CONSUL_LICENSE_PATH:-}"
RUN_DIR="${RUN_DIR:-$(cd "$(dirname "$0")" && pwd)/.run}"
PID_FILE="$RUN_DIR/agent.pid"
LOG_FILE="$RUN_DIR/agent.log"

up() {
  mkdir -p "$RUN_DIR"
  [[ -n "$CONSUL_BIN" ]] || { echo "CONSUL_BIN not set — point it at a Consul Enterprise binary"; exit 1; }
  command -v "$CONSUL_BIN" >/dev/null || { echo "consul binary not found at CONSUL_BIN=$CONSUL_BIN"; exit 1; }
  [[ -n "$LICENSE_PATH" ]] || { echo "CONSUL_LICENSE_PATH not set — point it at your .hclic license file"; exit 1; }
  [[ -f "$LICENSE_PATH" ]] || { echo "license not found at CONSUL_LICENSE_PATH=$LICENSE_PATH"; exit 1; }
  down || true
  CONSUL_LICENSE_PATH="$LICENSE_PATH" "$CONSUL_BIN" agent -dev -node node-primary -bind 127.0.0.1 -client 127.0.0.1 -dns-port=-1 \
    > "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  echo "consul enterprise dev agent started (PID $(cat "$PID_FILE")) — logs: $LOG_FILE"
}

down() {
  if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    kill "$(cat "$PID_FILE")" && echo "stopped agent $(cat "$PID_FILE")"
  fi
  rm -f "$PID_FILE"
}

case "${1:-up}" in
  up) up ;;
  down) down ;;
  *) echo "usage: $0 {up|down}"; exit 1 ;;
esac
