#!/usr/bin/env bats

load helpers

@test "gateway-primary proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "gateway-primary should have healthy endpoints for secondary" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 secondary HEALTHY 1
}

@test "gateway-primary should have healthy endpoints for secondary servers" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 server.secondary.consul HEALTHY 1
}

@test "gateway-primary should have healthy endpoints for lone primary server" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 pri.server.primary.consul HEALTHY 1
}

@test "gateway-secondary should be up and listening" {
  retry_long nc -z consul-secondary-client:4432
}

@test "primary should be able to rpc to the secondary" {
  retry_long curl -sL -f -XPUT localhost:8500/v1/kv/foo?dc=secondary -d'{"Value":"bar"}'
}

@test "wan pool should show 2 healthy nodes" {
  assert_alive_wan_member_count primary 2
}
