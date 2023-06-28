#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have a conflicted status" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted True RouteConflict primary api-gateway api-gateway
}

@test "api gateway should have no healthy endpoints for s1" {
  assert_upstream_missing 127.0.0.1:20000 s1
}

@test "api gateway should have no healthy endpoints for s2" {
  assert_upstream_missing 127.0.0.1:20000 s2
}