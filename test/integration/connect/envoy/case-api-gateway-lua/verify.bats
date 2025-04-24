#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_long curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway should have healthy endpoints for s1" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-one
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api-gateway should return 200 with custom message for non-existent path" {
  run retry_default curl -s -d "hello" "localhost:8080/nonexistent"
  echo "[DEBUG] response: $output" >&3

  [ "$status" == "0" ]
  echo "$output" | grep "Response modified by Lua script"
}

@test "api-gateway should return 200 for valid path" {
  run retry_default curl -s -f -d "hello" "localhost:8080/echo"
  [ "$status" == "0" ]
  [ "$output" == "hello" ]
}

@test "api-gateway should have lua filter configured" {
  FILTERS=$(get_envoy_http_filters localhost:19000)
  echo "[DEBUG] filters: $FILTERS" >&3

  echo "$FILTERS" | grep "envoy.filters.http.lua"
}

@test "api gateway should add Lua header when connecting to s1" {
  run retry_long sh -c "curl -s -D - localhost:9999/ | grep x-lua-added"
  [ "$status" -eq 0 ]
  [[ "$output" == "x-lua-added: test-value" ]]
}