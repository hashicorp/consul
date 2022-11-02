#!/usr/bin/env bats

load helpers

@test "gateway-secondary proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "gateway-secondary should have healthy endpoints for primary" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:19001 primary HEALTHY 1
}

@test "gateway-secondary should have healthy endpoints for primary servers" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:19001 server.primary.consul HEALTHY 1
}

@test "gateway-secondary should have healthy endpoints for lone secondary server" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19001 sec.server.secondary.consul HEALTHY 1
}

@test "gateway-primary should be up and listening" {
  retry_long nc -z consul-primary-client:4431
}

@test "secondary should be able to rpc to the primary" {
  retry_default curl -sL -f -XPUT localhost:8500/v1/kv/oof?dc=primary -d'{"Value":"rab"}'
}

@test "wan pool should show 2 healthy nodes" {
  assert_alive_wan_member_count secondary 2
}
