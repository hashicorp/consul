#!/usr/bin/env bats

load helpers

@test "terminating-gateway-primary proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "terminating-gateway-primary listener is up on :4431" {
  retry_default nc -z localhost:4431
}
