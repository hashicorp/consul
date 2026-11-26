#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# Local-binary fallback runner (RUNTIME=local). Mirrors the docker-compose
# topology using a locally-installed `consul` binary instead of a container.
#
#   ./local.sh up     # start a dev agent on :8500 (seeding is done by seed.ts)
#   ./local.sh down   # stop it
#
# The scenario CLI runs `seed.ts` separately, so this only manages the agent.
set -euo pipefail

CONSUL_BIN="${CONSUL_BIN:-consul}"
RUN_DIR="${RUN_DIR:-$(cd "$(dirname "$0")" && pwd)/.run}"
PID_FILE="$RUN_DIR/agent.pid"
LOG_FILE="$RUN_DIR/agent.log"

up() {
  mkdir -p "$RUN_DIR"
  command -v "$CONSUL_BIN" >/dev/null || { echo "consul binary not found (set CONSUL_BIN)"; exit 1; }
  down || true
  "$CONSUL_BIN" agent -dev -node node-primary -bind 127.0.0.1 -client 127.0.0.1 -dns-port=-1 \
    > "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  echo "consul dev agent started (PID $(cat "$PID_FILE")) — logs: $LOG_FILE"
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
