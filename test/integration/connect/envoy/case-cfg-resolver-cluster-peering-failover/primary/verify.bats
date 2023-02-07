#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "gateway-primary proxy admin is up on :19001" {
  retry_default curl localhost:19000/config_dump
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary
}

@test "s2 proxies should be healthy in alpha" {
  assert_service_has_healthy_instances s2 1 alpha
}

@test "gateway-alpha should be up and listening" {
  retry_long nc -z consul-alpha-client:4432
}

@test "peer the two clusters together" {
  retry_default create_peering primary alpha
}

@test "s2 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary "" "" primary-to-alpha
}

# Failover

@test "s1 upstream should have healthy endpoints for s2 in both primary and failover" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 failover-target~s2.default.primary.internal HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 failover-target~s2.default.primary-to-alpha.external HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s1 upstream made 1 connection" {
  assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.failover-target~s2.default.primary.internal.*cx_total" 1
}

@test "terminate instance of s2 primary envoy which should trigger failover to s2 alpha when the tcp check fails" {
  kill_envoy s2 primary
}

@test "s2 proxies should be unhealthy in primary" {
  assert_service_has_healthy_instances s2 0 primary
}

@test "s1 upstream should have healthy endpoints for s2 in the failover cluster peer" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 failover-target~s2.default.primary.internal UNHEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 failover-target~s2.default.primary-to-alpha.external HEALTHY 1
}

@test "reset envoy statistics for failover" {
  reset_envoy_metrics 127.0.0.1:19000
}

@test "gateway-alpha should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 exported~s2.default.alpha HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 in the failover cluster peer" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s1 upstream made 1 connection to s2 through the cluster peer" {
  assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.failover-target~s2.default.primary-to-alpha.external.*cx_total" 1
}

# Redirect

@test "reset envoy statistics for redirect" {
  reset_envoy_metrics 127.0.0.1:19000
}

@test "s1 upstream should have healthy endpoints for s2 (virtual-s2) in the cluster peer" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary-to-alpha.external HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 via virtual-s2" {
  run retry_default curl -s -f -d hello localhost:5001
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s1 upstream made 1 connection to s2 via virtual-s2 through the cluster peer" {
  assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.s2.default.primary-to-alpha.external.*cx_total" 1
}
