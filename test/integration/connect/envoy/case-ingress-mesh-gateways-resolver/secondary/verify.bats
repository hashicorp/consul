#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19001
}

@test "s2 proxy is running correct version" {
  assert_envoy_version 19002
}

@test "s1 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2 proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats -o /dev/null
}

@test "gateway-secondary proxy admin is up on :19003" {
  retry_default curl -f -s localhost:19003/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1 secondary
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2 secondary
}

@test "s1 proxy should be healthy" {
  assert_service_has_healthy_instances s1 1 secondary
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1 secondary
}

@test "gateway-secondary is used for the upstream connection for s1" {
  assert_envoy_metric_at_least 127.0.0.1:19003 "cluster.s1.default.secondary.*cx_total" 1
}

@test "gateway-secondary is used for the upstream connection for s2" {
  assert_envoy_metric_at_least 127.0.0.1:19003 "cluster.s2.default.secondary.*cx_total" 1
}
