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
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 ef15b5b5~s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 via grpc" {
  # This test also covers http2 since gRPC always uses http2
  run fortio grpcping localhost:5000

  echo "OUTPUT: $output"

  [ "$status" == 0 ]
}

@test "s1 proxy should be sending gRPC metrics to statsd" {
  # in envoy 1.18.x the format of the emitted grpc metrics changed slightly
  metrics_query='envoy.cluster.grpc.fgrpc.PingServer.Ping.total.*[#,]local_cluster:s1(,|$)'
  if [[ "${ENVOY_VERSION}" =~ ^1\.1[567]\. ]]; then
    metrics_query='envoy.cluster.grpc.PingServer.total.*[#,]local_cluster:s1(,|$)'
  fi

  run retry_default must_match_in_statsd_logs "${metrics_query}"
  echo "OUTPUT: $output"

  [ "$status" == 0 ]
}
