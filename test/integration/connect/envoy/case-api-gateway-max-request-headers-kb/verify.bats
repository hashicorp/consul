#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway route should be bound" {
  assert_config_entry_status Bound True Bound primary http-route my-gateway-route
}

@test "api gateway should have healthy endpoints for s1" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway should be able to connect to s1 with normal headers" {
  run retry_long curl -s -w "HTTP_CODE:%{http_code}" -d "hello" localhost:9999
  echo "Normal headers - Status: $status, Output: $output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should accept requests with large headers under limit" {
  # Create header data well under 96KB limit (~30KB to account for other headers and overhead)
  local header_value=$(printf "A%.0s" {1..30000})
  run retry_long curl -s -H "X-Large-Header: $header_value" -d "hello" localhost:9999
  echo "Large headers under limit - Status: $status, Output: $output" >&3
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should reject requests with headers over limit" {
  # Create header data over 96KB limit (~100KB)
  local header_value=$(printf "A%.0s" {1..102000})
  run curl -s -w "HTTP_CODE:%{http_code}" -H "X-Large-Header: $header_value" -d "hello" localhost:9999
  echo "Status: $status, Output: $output" >&3
  [ "$status" -ne 0 ] || [[ "$output" == *"431"* ]] || [[ "$output" == *"400"* ]]
}