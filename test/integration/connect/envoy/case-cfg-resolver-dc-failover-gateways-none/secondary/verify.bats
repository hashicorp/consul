#!/usr/bin/env bats

load helpers

@test "s2 proxy is running correct version" {
  assert_envoy_version 19002
}

@test "s2 proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats | grep cx_total
}

@test "s2 proxy admin is up on :19002 and has metrics" {
  wait_for_envoy_metric_to_exist 127.0.0.1:19002 "cluster.s2.default.secondary.*cx_total"
}

@test "gateway-secondary proxy admin is up on :19003" {
  retry_default curl -f -s localhost:19003/stats | grep cx_total
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1 secondary
}

@test "gateway-secondary stats should exist" {
  wait_for_envoy_metric_to_exist 127.0.0.1:19003 "cluster.s2.default.secondary.*cx_total"
}

@test "gateway-secondary stats should be 0 at startup" {
  assert_envoy_metric 127.0.0.1:19003 "cluster.s2.default.secondary.*cx_total" 0
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s2 secondary
}

@test "gateway-secondary is NOT used for the upstream connection" {
  assert_envoy_metric 127.0.0.1:19003 "cluster.s2.default.secondary.*cx_total" 0
}
