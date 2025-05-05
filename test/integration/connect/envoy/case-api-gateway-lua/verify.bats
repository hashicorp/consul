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

@test "api-gateway should have lua filter configured" {
  FILTERS=$(get_envoy_http_filters localhost:20000)
  echo "$FILTERS" | grep "envoy.filters.http.lua"
}
@test "s1 should have lua filter configured" {
  FILTERS=$(get_envoy_http_filters localhost:19000)
  echo "$FILTERS" | grep "envoy.filters.http.lua"
}

@test "api-gateway should add envoy headers" {
  run retry_default curl -i localhost:9999/echo
  echo "[DEBUG] response: $output" >&3
  [ "$status" == "0" ]
  # Extract headers (before the first blank line) and body (after the blank line)
  headers=$(echo "$output" | sed '/^\s*$/q')
  body=$(echo "$output" | sed '1,/^\s*$/d')

  echo "[DEBUG] headers: $headers" >&3
  echo "[DEBUG] body: $body" >&3

  # Check if the headers contains the header on added by the Lua script in the request
  onrequest=$(echo "$body" | grep -i 'x-lua-added-onrequest')

  # Check if the body contains the header on added by the Lua script in the response
  onresponse=$(echo "$headers" | grep -i 'x-lua-added-onresponse')

  [[ -z "$onrequest" ]] && echo "x-lua-added-onrequest not found" >&3 && return 1
  [[ -z "$onresponse" ]] && echo "x-lua-added-onresponse not found" >&3 && return 1
}