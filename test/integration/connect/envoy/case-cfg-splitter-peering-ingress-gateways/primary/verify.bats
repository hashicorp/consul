#!/usr/bin/env bats

load helpers

@test "ingress-primary proxy admin is up" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "s1 proxy admin is up" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "services should be healthy in primary" {
  assert_service_has_healthy_instances s1 1 primary
}

@test "services should be healthy in alpha" {
  assert_service_has_healthy_instances s1 1 alpha
  assert_service_has_healthy_instances s2 1 alpha
}

@test "mesh-gateway should have healthy endpoints" {
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 s1 HEALTHY 1
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 s2 HEALTHY 1
}

@test "peer the two clusters together" {
  retry_long create_peering primary alpha
}

@test "s1, s2 alpha proxies should be imported to primary" {
  retry_long assert_service_has_imported primary s1 primary-to-alpha
  retry_long assert_service_has_imported primary s2 primary-to-alpha
}

@test "s1 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s1 1 primary "" "" primary-to-alpha
}

@test "s2 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary "" "" primary-to-alpha
}

@test "ingress should have healthy endpoints to alpha s1 and s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1.default.primary-to-alpha.external HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2.default.primary-to-alpha.external HEALTHY 1
}

@test "requests through ingress should proxy to alpha" {
  assert_expected_fortio_name s1-alpha peer-s1.ingress.consul 10000
  assert_expected_fortio_name s2-alpha peer-s2.ingress.consul 9999
}

@test "ingress made 1 connection to alpha s1" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.s1.default.primary-to-alpha.external.*cx_total" 1
}

@test "ingress made 1 connection to alpha s2" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "cluster.s2.default.primary-to-alpha.external.*cx_total" 1
}

@test "no requests contacted primary s1" {
  assert_envoy_metric 127.0.0.1:19000 "http.public_listener.rq_total" 0
}

@test "requests through ingress should proxy to primary s1" {
  assert_expected_fortio_name s1 s1.ingress.consul 10001
  assert_envoy_metric 127.0.0.1:19000 "http.public_listener.rq_total" 1
}

@test "requests through ingress to splitter should go to alpha" {
  retry_long assert_expected_fortio_name s1-alpha split.ingress.consul 10002
  retry_long assert_expected_fortio_name s2-alpha split.ingress.consul 10002
}

