#!/usr/bin/env bats

load helpers

@test "s2 proxy is running correct version" {
  assert_envoy_version 19001
}

@test "ingress-primary proxy admin is up" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "services should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 alpha
}

@test "services should be healthy in alpha" {
  assert_service_has_healthy_instances s2 1 alpha
}

@test "mesh-gateway should have healthy endpoints" {
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 s2 HEALTHY 1
}

@test "peer the two clusters together" {
  retry_default create_peering primary alpha
}

@test "s2 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary "" "" primary-to-alpha
}

# Failover

@test "s1 upstream should have healthy endpoints for s2 in both primary and failover" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 failover-target~0~s2.default.primary.internal HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 failover-target~1~s2.default.primary.internal HEALTHY 1
}

@test "ingress-gateway should be able to connect to s2" {
  assert_expected_fortio_name s2 127.0.0.1 10000
}

@test "s1 upstream made 1 connection" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.failover-target~0~s2.default.primary.internal.*cx_total" 1
}

@test "terminate instance of s2 primary envoy which should trigger failover to s2 alpha when the tcp check fails" {
  kill_envoy s2 primary
}

@test "s2 proxies should be unhealthy in primary" {
  assert_service_has_healthy_instances s2 0 primary
}

@test "s1 upstream should have healthy endpoints for s2 in the failover cluster peer" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 failover-target~0~s2.default.primary.internal UNHEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 failover-target~1~s2.default.primary.internal HEALTHY 1
}

@test "reset envoy statistics for failover" {
  reset_envoy_metrics 127.0.0.1:20000
}

@test "gateway-alpha should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 exported~s2.default.alpha HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 in the failover cluster peer" {
  assert_expected_fortio_name s2-alpha 127.0.0.1 10000
}

@test "s1 upstream made 1 connection to s2 through the cluster peer" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.failover-target~1~s2.default.primary.internal.*cx_total" 1
}
