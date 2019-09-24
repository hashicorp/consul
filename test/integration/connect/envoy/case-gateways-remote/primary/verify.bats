#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s1 upstream should have healthy endpoints for s2" {
  # mesh gateway mode is configured in an upstream override so the cluster name is customized here
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 dd412229~s2.default.secondary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "s1 upstream made 1 connection" {
  assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.dd412229~s2.default.secondary.*cx_total" 1
}
