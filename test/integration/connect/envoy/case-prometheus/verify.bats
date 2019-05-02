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

@test "s1 upstream should be able to connect to s2 with http/1.1" {
  run retry_default curl --http1.1 -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "s1 proxy should be exposing metrics to prometheus" {
  # Should have http metrics. This is just a sample one. Require the metric to
  # be present not just found in a comment (anchor the regexp).
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '^envoy_http_downstream_rq_active'

  # Should be labelling with local_cluster.
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '[\{,]local_cluster="s1"[,}] '

  # Should be labelling with http listener prefix.
  retry_default \
    must_match_in_prometheus_response localhost:1234 \
    '[\{,]envoy_http_conn_manager_prefix="public_listener_http"[,}]'
}