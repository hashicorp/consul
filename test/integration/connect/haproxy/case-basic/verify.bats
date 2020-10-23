#!/usr/bin/env bats

load helpers

# @test "s1 proxy is running correct version" {
#   assert_haproxy_connect_version basic
# }

@test "s1 proxy admin is up on :8080" {
 retry_default curl -f -s localhost:8080 -o /dev/null
}

@test "s2 proxy admin is up on :8181" {
 retry_default curl -f -s localhost:8181 -o /dev/null
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

# This is specific to Envoy
# @test "s1 upstream should have healthy endpoints for s2" {
#   assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
# }

@test "s1 upstream should be able to connect to s2" {
  run curl -s -f -d hello localhost:5000

  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

# @test "s1 proxy should have been configured with one rbac listener filter at L4" {
#   LISTEN_FILTERS=$(get_envoy_listener_filters localhost:19000)
#   PUB=$(echo "$LISTEN_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ' )
#   UPS=$(echo "$LISTEN_FILTERS" | grep -E "^s2:" | cut -f 2 -d ' ' )

#   echo "LISTEN_FILTERS = $LISTEN_FILTERS"
#   echo "PUB = $PUB"
#   echo "UPS = $UPS"

#   [ "$PUB" = "envoy.filters.network.rbac,envoy.tcp_proxy" ]
#   [ "$UPS" = "envoy.tcp_proxy" ]
# }
