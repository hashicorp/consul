#!/usr/bin/env bats

load helpers

# A TLS 1.3 minimum without a maximum must still allow TLS 1.3 upstream
# connections (Envoy clients otherwise default to a TLS 1.2 maximum), and
# Consul injects the hybrid post-quantum group by default.

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

@test "s1 to s2 handshake negotiated the X25519MLKEM768 key exchange group" {
  retry_default assert_envoy_metric_at_least 127.0.0.1:19000 "s2.default.primary.*ssl.curves.X25519MLKEM768:" 1
}
