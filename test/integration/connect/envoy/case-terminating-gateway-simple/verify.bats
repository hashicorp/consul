#!/usr/bin/env bats

load helpers

@test "terminating proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "terminating-gateway-primary listener is up on :8443" {
  retry_default nc -z localhost:8443
}

@test "terminating-gateway should have healthy endpoints for s2" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "terminating-gateway is used for the upstream connection" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "s2.default.primary.*cx_total" 1
}
