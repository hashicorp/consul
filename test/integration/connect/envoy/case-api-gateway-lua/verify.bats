#!/usr/bin/env bats

load helpers

@test "api-gateway proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "static-server proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "api-gateway listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 api-gateway
}

@test "static-server listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 static-server
}

@test "api-gateway should be healthy" {
  assert_service_has_healthy_instances api-gateway 1
}

@test "static-server should be healthy" {
  assert_service_has_healthy_instances static-server 1
}

@test "api-gateway upstream should have healthy endpoints for static-server" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 static-server.default.primary HEALTHY 1
}

@test "api-gateway should return 200 with custom message for non-existent path" {
  run retry_default curl -s -d "hello" "localhost:8080/nonexistent"
  [ "$status" == "0" ]
  echo "$output" | grep "Please check whether page or URI is configured correctly or not for api gateway"
}

@test "api-gateway should return 200 for valid path" {
  run retry_default curl -s -f -d "hello" "localhost:8080/echo"
  [ "$status" == "0" ]
  [ "$output" == "hello" ]
}

@test "api-gateway should have lua filter configured" {
  FILTERS=$(get_envoy_listener_filters localhost:19000)
  echo "$FILTERS" | grep "envoy.filters.http.lua"
}

@test "api-gateway lua extension" {
  # Wait for services to be registered
  retry_default curl -s -f localhost:8500/v1/catalog/service/static-server > /dev/null
  retry_default curl -s -f localhost:8500/v1/catalog/service/api-gateway > /dev/null

  # Wait for API Gateway to be ready
  retry_default curl -s -f localhost:8080/health > /dev/null

  # Test that the LUA extension adds the x-test header
  run curl -s -v localhost:8080/health
  [ "$status" -eq 0 ]
  echo "$output" | grep "x-test: test"
} 