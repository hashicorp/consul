#!/usr/bin/env bats

load helpers

@test "s1 has lambda cluster for l1" {
  assert_lambda_envoy_dynamic_cluster_exists localhost:19000 l1 us-west-2 443
}

@test "s1 has lambda http filter for l1" {
  assert_lambda_envoy_dynamic_http_filter_exists localhost:19000 l1 arn:aws:lambda:us-west-2:977604411308:function:consul-envoy-integration-test true null
}

@test "terminating gateway has lambda cluster for l2" {
  assert_lambda_envoy_dynamic_cluster_exists localhost:20000 l2 us-west-2 443
}

@test "terminating gateway has lambda http filter for l2" {
  assert_lambda_envoy_dynamic_http_filter_exists localhost:20000 l2 arn:aws:lambda:us-west-2:977604411308:function:consul-envoy-integration-test null null
}

@test "s1 can call l1 through its sidecar-proxy" {
  run retry_default curl -s -f -H "Content-type: application/json" -d '"hello"' 'localhost:1234'
  [ "$status" -eq 0 ]

  # l1 is configured with payload_passthrough = true so the response needs to be unwrapped
  [ $(echo "$output" | jq -r '.statusCode') -eq 200 ]
  [ $(echo "$output" | jq -r '.body') == "hello" ]
}

@test "s1 can call l2 through the terminating gateway" {
  run retry_default curl -s -f -H "Content-type: application/json" -d '"hello"' 'localhost:5678'
  [ "$status" -eq 0 ]
  [ "$output" == '"hello"' ]

   # Omitting the Content-type in the request will cause envoy to base64 encode the request.
  run curl -s -f -d '{"message":"hello"}' 'localhost:5678'
  [ "$status" -eq 0 ]
  [ "$output" == '{"message":"hello"}' ]
}
