#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway should have healthy endpoints for s1" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-one
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api-gateway should have lua filter configured" {
  FILTERS=$(get_envoy_http_filters localhost:20000)
  echo "$FILTERS" | grep "envoy.filters.http.lua"
}
@test "s1 should have lua filter configured" {
  FILTERS=$(get_envoy_http_filters localhost:19000)
  echo "$FILTERS" | grep "envoy.filters.http.lua"
}

@test "api-gateway should add header on response in LUA script" {
  run retry_default curl -s -D - localhost:9999/echo
  [ "$status" -eq 0 ]
  echo "$output" | grep -i "x-lua-added-onresponse"
}

@test "api-gateway should add header in request in LUA script" {
  run retry_default curl -s localhost:9999/echo
  [ "$status" -eq 0 ]
  echo "$output" | grep -i "x-lua-added-onrequest"
}