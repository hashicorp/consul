#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

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


@test "s1 proxy should have max_request_headers_kb set to 96" {
  assert_envoy_max_request_headers_kb 127.0.0.1:19000 96
}

@test "s2 proxy should have max_request_headers_kb set to 96" {
  assert_envoy_max_request_headers_kb 127.0.0.1:19001 96
}

@test "s1 upstream should work with normal headers" {
  run retry_default curl -s -f -d hello 127.0.0.1:5000
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s2 should accept requests with headers under 96KB limit (95KB test)" {
  # Create a header with 95KB of data (under the limit)
  large_header=$(head -c 95000 < /dev/zero | tr '\0' 'a')
  
  run retry_default curl -s -f -H "X-Custom-Header: $large_header" -d "test-payload" 127.0.0.1:5000/
  [ "$status" -eq 0 ]
  [[ "$output" == *"test-payload"* ]]
}

@test "s2 should reject requests with headers over 96KB limit (99KB test)" {
  # Create a header with 99KB of data (over the limit)
  large_header=$(head -c 99000 < /dev/zero | tr '\0' 'a')
  
  # Test should return error response for headers exceeding limit
  run curl -s -w "%{http_code}" -H "X-Custom-Header: $large_header" -d "test-payload" 127.0.0.1:5000/ -o /dev/null
  [ "$status" -eq 0 ]
  # Should get HTTP 431 (Request Header Fields Too Large) or similar error code
  [[ "$output" == *"431"* ]]
}