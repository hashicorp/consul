#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

readonly START_LOG_FILE="${CONSUL_UI_TESTING_START_LOG_FILE:-consul-start.log}"
readonly START_PID_FILE="${CONSUL_UI_TESTING_START_PID_FILE:-consul-start.pid}"
readonly PEERING_SUCCESS_TEXT="We have successfully setup peering"

require_consul_image() {
  if [[ -z "${CONSUL_IMAGE:-}" ]]; then
    echo "::error::CONSUL_IMAGE must be set"
    exit 1
  fi
}

startup_pid() {
  cat "$START_PID_FILE"
}

print_docker_state() {
  echo "--- docker compose ps ---"
  docker compose ps || true
  echo "--- docker ps -a ---"
  docker ps -a || true
}

fail_with_startup_logs() {
  local message="$1"
  local log_lines="$2"

  echo "::error::${message}"
  echo "--- Last ${log_lines} lines of ${START_LOG_FILE} ---"
  tail -n "$log_lines" "$START_LOG_FILE" || true
  print_docker_state
  exit 1
}

start_servers() {
  local edition_label="$1"

  require_consul_image

  echo "Starting Consul API servers (${edition_label}) at $(date '+%Y-%m-%d %H:%M:%S')"
  yarn start "$CONSUL_IMAGE" > "$START_LOG_FILE" 2>&1 &
  local pid=$!
  printf '%s\n' "$pid" > "$START_PID_FILE"

  echo "consul-ui-testing start PID: $(startup_pid)"
  echo "Process table entry for startup command:"
  ps -fp "$(startup_pid)" || true

  sleep 10
  if ! kill -0 "$(startup_pid)" 2>/dev/null; then
    echo "--- ${START_LOG_FILE} ---"
    tail -n 200 "$START_LOG_FILE" || true
    echo "--- docker ps -a ---"
    docker ps -a || true
    echo "::error::Consul startup process exited within the first 10 seconds"
    exit 1
  fi

  echo "Startup process is still running after 10 seconds"
  echo "--- Initial ${START_LOG_FILE} (last 100 lines) ---"
  tail -n 100 "$START_LOG_FILE" || true
  print_docker_state
}

wait_for_peering() {
  local last_log_line=0

  echo "Waiting up to 5 minutes for Consul peering setup..."
  for i in $(seq 1 60); do
    local current_log_line
    current_log_line=$(wc -l < "$START_LOG_FILE" 2>/dev/null || echo 0)

    if [[ "$current_log_line" -gt "$last_log_line" ]]; then
      echo "--- New ${START_LOG_FILE} output (lines $((last_log_line + 1))-${current_log_line}) ---"
      sed -n "$((last_log_line + 1)),${current_log_line}p" "$START_LOG_FILE" || true
      last_log_line=$current_log_line
    fi

    if grep -q "$PEERING_SUCCESS_TEXT" "$START_LOG_FILE" 2>/dev/null; then
      echo "Consul peering is ready (after ~$((i * 5))s)"
      echo "--- docker compose ps at peering completion ---"
      docker compose ps || true
      return 0
    fi

    if ! kill -0 "$(startup_pid)" 2>/dev/null; then
      fail_with_startup_logs "Consul startup process exited before peering completed" 300
    fi

    if (( i % 6 == 0 )); then
      echo "Still waiting... ($((i * 5))s elapsed)"
      echo "--- docker compose ps ---"
      docker compose ps || true
    fi

    sleep 5
  done

  fail_with_startup_logs "Timed out waiting for Consul peering setup (300s)" 300
}

verify_api_readiness() {
  echo "Verifying Consul API readiness before starting UI..."
  echo "--- docker compose ps ---"
  docker compose ps || true

  for i in $(seq 1 12); do
    local primary_leader secondary_leader
    primary_leader=$(curl -fsS http://localhost:8500/v1/status/leader || true)
    secondary_leader=$(curl -fsS http://localhost:8501/v1/status/leader || true)

    if [[ -n "$primary_leader" && -n "$secondary_leader" && "$primary_leader" != '""' && "$secondary_leader" != '""' ]]; then
      echo "Primary leader: $primary_leader"
      echo "Secondary leader: $secondary_leader"
      return 0
    fi

    echo "Consul APIs not ready yet ($((i * 5))s elapsed)"
    echo "Primary leader response: ${primary_leader:-<empty>}"
    echo "Secondary leader response: ${secondary_leader:-<empty>}"
    sleep 5
  done

  fail_with_startup_logs "Consul HTTP APIs did not become ready in time" 200
}

report_container_states() {
  echo "Container states after Consul readiness:"
  docker ps -a --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'
  echo "Exited containers:"
  docker ps -a --filter status=exited --format '{{.Names}} {{.Status}}' || true

  local container
  for container in product-api public-api payments product-db product-api-secondary public-api-secondary payments-secondary product-db-secondary frontend frontend-secondary; do
    if docker ps -a --format '{{.Names}}' | grep -qx "$container"; then
      echo "--- Last 40 log lines for $container ---"
      docker logs --tail 40 "$container" || true
    fi
  done
}

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-30}"
  local interval_seconds="${4:-2}"

  echo "Waiting for ${label} on ${url}..."
  for i in $(seq 1 "$attempts"); do
    if curl -sf "$url" > /dev/null 2>&1; then
      echo "${label} is ready (after ~$((i * interval_seconds))s)"
      return 0
    fi

    echo "Waiting for ${label}... ($((i * interval_seconds))s elapsed)"
    sleep "$interval_seconds"
  done

  echo "::error::Timed out waiting for ${label} ($((attempts * interval_seconds))s)"
  exit 1
}

main() {
  local command="${1:-}"

  case "$command" in
    start)
      start_servers "${2:-}"
      ;;
    wait-for-peering)
      wait_for_peering
      ;;
    verify-api)
      verify_api_readiness
      ;;
    report-containers)
      report_container_states
      ;;
    wait-for-url)
      if [[ $# -lt 3 ]]; then
        echo "Usage: $0 wait-for-url <url> <label> [attempts] [interval_seconds]"
        exit 1
      fi
      wait_for_url "$2" "$3" "${4:-30}" "${5:-2}"
      ;;
    *)
      echo "Usage: $0 {start <edition>|wait-for-peering|verify-api|report-containers|wait-for-url <url> <label> [attempts] [interval_seconds]}"
      exit 1
      ;;
  esac
}

main "$@"