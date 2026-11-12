#!/usr/bin/env bats

load helpers

# A client that only offers P-256 must be accepted by a server that uses the
# automatically injected default key exchange groups.

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s1 to s2 handshake negotiated TLSv1.3" {
  retry_default assert_envoy_metric_at_least 127.0.0.1:19000 "s2.default.primary.*ssl.versions.TLSv1.3" 1
}

@test "s1 to s2 handshake negotiated the P-256 key exchange group" {
  retry_default assert_envoy_metric_at_least 127.0.0.1:19000 "s2.default.primary.*ssl.curves.P-256:" 1
}
