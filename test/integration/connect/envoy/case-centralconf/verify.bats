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
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2 with http/1.1" {
  run retry_default curl --http1.1 -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "s1 proxy should be exposing metrics to prometheus from central config" {
  # Should have http metrics. This is just a sample one. Require the metric to
  # be present not just found in a comment (anchor the regexp).
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '^envoy_http_downstream_rq_active'

  # Should be labelling with local_cluster.
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '[\{,]local_cluster="s1"[,}] '

  # Ensure we have http metrics for public listener
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '[\{,]envoy_http_conn_manager_prefix="public_listener_http"[,}]'

  # Ensure we have http metrics for s2 upstream
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '[\{,]envoy_http_conn_manager_prefix="upstream_s2_http"[,}]'
}
