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

@test "s1 upstream should be able to connect to s2 with http/1.1" {
  run retry_default curl --http1.1 -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

@test "s1 proxy should be exposing the /stats prefix" {
  # Should have http metrics. This is just a sample one. Require the metric to
  # be present not just found in a comment (anchor the regexp).
  retry_default \
    must_match_in_stats_proxy_response localhost:1239 \
    'stats' '^http.envoy_metrics.downstream_rq_active'

  # Response should include the the local cluster request.
  retry_default \
    must_match_in_stats_proxy_response localhost:1239 \
    'stats' 'cluster.local_agent.upstream_rq_active'

  # Response should include the http public listener.
  retry_default \
     must_match_in_stats_proxy_response localhost:1239 \
    'stats' 'http.public_listener'

  # /stats/prometheus should also be reachable and labelling the local cluster.
  retry_default \
     must_match_in_stats_proxy_response localhost:1239 \
    'stats/prometheus' '[\{,]consul_source_service="s1"[,}]'

  # /stats/prometheus should also be reachable and exposing metrics.
  retry_default \
     must_match_in_stats_proxy_response localhost:1239 \
    'stats/prometheus' 'envoy_http_downstream_rq_active'
}
