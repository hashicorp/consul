#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

@test "test lua adding a header" {
  run retry_default curl -s -f \
    "localhost:5000/debug?env=dump"

  [ "$status" == "0" ]

  echo "$output" | grep -E "X-Consul-Service: s1"
  echo "$output" | grep -E "X-Consul-Datacenter: primary"
  echo "$output" | grep -E "X-Consul-Namespace: default"
  echo "$output" | grep -E "X-Consul-Trust-Domain: (\w+-){4}\w+.consul"
}

@test "s1(tcp) proxy should not be changed by lua extension" {
  TCP_FILTERS=$(get_envoy_listener_filters localhost:19000)
  PUB=$(echo "$TCP_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ')

  echo "TCP_FILTERS = $TCP_FILTERS"
  echo "PUB = $PUB"

  [ "$PUB" = "envoy.filters.network.rbac,envoy.filters.network.tcp_proxy" ]
}
