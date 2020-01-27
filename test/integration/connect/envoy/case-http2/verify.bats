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
  # protocol is configured in an upstream override so the cluster name is customized here
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 49c19fe6~s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 via http2" {
  # We use grpc here because it's the easiest way to test http2. The server
  # needs to support h2c since the proxy doesn't talk TLS to the local app.
  # Most http2 servers don't support that but gRPC does. We could use curl
  run curl -f -s -X POST localhost:5000/PingServer.Ping/

  echo "OUTPUT: $output"

  [ "$status" == 0 ]
}
