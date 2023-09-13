#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have be accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway should be bound to route one" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-one
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway should be bound to route two" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-two
}

@test "api gateway should be unbound to route three" {
  assert_config_entry_status Bound False FailedToBind primary http-route api-gateway-route-three
}

@test "api gateway should be bound to route four" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-four
}

@test "api gateway should be bound to route five" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-five
}

@test "api gateway should be able to connect to s1 via route one with the proper host" {
  run retry_long curl -H "Host: test.consul.example" -s -f -d hello localhost:9999
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should not be able to connect to s1 via route one with a mismatched host" {
  run retry_default sh -c "curl -H \"Host: foo.consul.example\" -sI -o /dev/null -w \"%{http_code}\" localhost:9999 | grep 404"
  [ "$status" -eq 0 ]
  [[ "$output" == "404" ]]
}

@test "api gateway should be able to connect to s1 via route two with the proper host" {
  run retry_long curl -H "Host: foo.bar.baz" -s -f -d hello localhost:9998
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should be able to connect to s1 via route four with any subdomain of the listener host" {
  run retry_long curl -H "Host: test.consul.example" -s -f -d hello localhost:9996
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
  run retry_long curl -H "Host: foo.consul.example"  -s -f -d hello localhost:9996
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway should be able to connect to s1 via route five with the proper host" {
  run retry_long curl -H "Host: foo.bar.baz" -s -f -d hello localhost:9995
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}