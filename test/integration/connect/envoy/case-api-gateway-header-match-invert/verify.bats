#!/usr/bin/env bats
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

load helpers

# ---------------------------------------------------------------------------
# Infrastructure health
# ---------------------------------------------------------------------------

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "http-route should be bound" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-invert
}

@test "api gateway should have healthy endpoints for s1 (canary)" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway should have healthy endpoints for s2 (default)" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

# ---------------------------------------------------------------------------
# Header-invert routing: X-Canary present + invert=true (rule 1)
# Requests WITHOUT the X-Canary header must reach s2 (the "default" backend).
# ---------------------------------------------------------------------------

@test "request without X-Canary header routes to s2 (default backend)" {
  # No header sent — rule 1 fires (present+invert=true matches absence).
  run retry_default curl -f -s localhost:9999/debug?env=dump
  [ "$status" -eq 0 ]
  echo "$output" | grep -E "^FORTIO_NAME=s2$"
}

# ---------------------------------------------------------------------------
# Header-invert routing: X-Canary present (rule 2, fallthrough)
# Requests WITH the X-Canary header must reach s1 (the "canary" backend).
# ---------------------------------------------------------------------------

@test "request with X-Canary header routes to s1 (canary backend)" {
  # Header present — rule 1 does NOT match (invert=true means absent), falls
  # through to rule 2 which matches the present header.
  run retry_default curl -f -s -H "x-canary: true" localhost:9999/debug?env=dump
  [ "$status" -eq 0 ]
  echo "$output" | grep -E "^FORTIO_NAME=s1$"
}

# ---------------------------------------------------------------------------
# Invert is header-value agnostic: any non-empty value still routes to s1.
# ---------------------------------------------------------------------------

@test "request with X-Canary set to arbitrary value still routes to s1" {
  run retry_default curl -f -s -H "x-canary: canary-pod-xyz" localhost:9999/debug?env=dump
  [ "$status" -eq 0 ]
  echo "$output" | grep -E "^FORTIO_NAME=s1$"
}
