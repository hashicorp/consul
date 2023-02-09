#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have healthy endpoints for s1" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway should be able to connect to s1 via configured port" {
  run retry_default curl -s -f -d hello localhost:9999
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}