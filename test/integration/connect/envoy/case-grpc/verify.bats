#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2 HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 via grpc" {
  run fortio grpcping localhost:5000

  echo "OUTPUT: $output"

  [ "$status" == 0 ]
}

@test "s1 proxy should be sending gRPC metrics to statsd" {
  run retry_default must_match_in_statsd_logs 'envoy.cluster.default.dc1.internal.*.consul.grpc.PingServer.total'

  echo "OUTPUT: $output"

  [ "$status" == 0 ]
}
