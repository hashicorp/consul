#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have be accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway should have healthy endpoints for s1" {
  assert_config_entry_status Bound True Bound primary tcp-route api-gateway-route-one
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway should have healthy endpoints for s2" {
  assert_config_entry_status Bound True Bound primary tcp-route api-gateway-route-two
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "api gateway should be able to connect to s1 via configured port" {
  run retry_default curl -s -f -d hello localhost:9999
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should get an intentions error connecting to s2 via configured port" {
  run retry_default must_fail_tcp_connection localhost:9998
}