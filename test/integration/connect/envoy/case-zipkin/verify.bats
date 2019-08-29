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
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 1a47f6e1~s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" == "0" ]
  [ "$output" == "hello" ]
}

@test "s1 proxy should send trace spans to zipkin/jaeger" {
  # Send traced request through upstream. Debug echoes headers back which we can
  # use to get the traceID generated (no way to force one I can find with Envoy
  # currently?)
  run curl -s -f -H 'x-client-trace-id:test-sentinel' localhost:5000/Debug

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  # Get the traceID from the output
  TRACEID=$(echo $output | grep 'X-B3-Traceid:' | cut -c 15-)

  # Get the trace from Jaeger. Won't bother parsing it just seeing it show up
  # there is enough to know that the tracing config worked.
  run retry_default curl -s -f "localhost:16686/api/traces/$TRACEID"
}
