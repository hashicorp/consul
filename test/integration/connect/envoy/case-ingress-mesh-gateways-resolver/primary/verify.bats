#!/usr/bin/env bats

load helpers

@test "gateway-primary proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats -o /dev/null
}

@test "ingress-primary proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "ingress should have healthy endpoints for s1" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1.default.secondary HEALTHY 1
}

@test "ingress should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2.default.secondary HEALTHY 1
}

@test "gateway-primary should have healthy endpoints for secondary" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:19002 secondary HEALTHY 1
}

@test "gateway-secondary should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status consul-secondary-client:19003 s1 HEALTHY 1
}

@test "gateway-secondary should have healthy endpoints for s2" {
   assert_upstream_has_endpoints_in_status consul-secondary-client:19003 s2 HEALTHY 1
}

@test "ingress should be able to connect to s1" {
  run retry_default curl -s -f -d hello localhost:10000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "ingress made 1 connection to s1" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.s1.default.secondary.*cx_total" 1
}

@test "gateway-primary is not used for the upstream connection to s1" {
  assert_envoy_metric 127.0.0.1:19002 "cluster.secondary.*cx_total" 0
}

@test "ingress should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:9999
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "ingress made 1 connection to s2" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.s2.default.secondary.*cx_total" 1
}

@test "gateway-primary is used for the upstream connection to s2" {
  assert_envoy_metric_at_least 127.0.0.1:19002 "cluster.secondary.*cx_total" 1
}
