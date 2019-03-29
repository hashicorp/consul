#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" == 0 ]
  [ "$output" == "hello" ]
}

@test "s1 proxy should be sending metrics to statsd" {
  run retry_default cat /workdir/statsd/statsd.log

  echo "METRICS:"
  echo "$output"
  echo "COUNT: $(echo "$output" | grep -Ec '^envoy\.')"

  [ "$status" == 0 ]
  [ $(echo $output | grep -Ec '^envoy\.') -gt "0" ]
}