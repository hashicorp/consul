#!/usr/bin/env bats

load helpers

@test "proxy admin endpoint is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20001" {
  retry_default curl -f -s localhost:20001/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20002" {
  retry_default curl -f -s localhost:20002/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20003" {
  retry_default curl -f -s localhost:20003/stats -o /dev/null
}

@test "proxy admin endpoint is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "random sampling with 100% should send traces to zipkin/jaeger" {
  run curl -s -f localhost:9991

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  TRACEID=$(echo $output | grep 'X-B3-Traceid:' | cut -c 15-)

  run retry_default curl -s -f "localhost:16686/api/traces/$TRACEID"
}

@test "client sampling with 0% should send traces to zipkin/jaeger" {
  run curl -s -f localhost:9992

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  TRACEID=$(echo $output | grep 'X-B3-Traceid:' | cut -c 15-)

  run retry_default curl -s -f "localhost:16686/api/traces/$TRACEID"
}

@test "random sampling with 0% should not send traces to zipkin/jaeger" {
  run curl -s -f localhost:9990

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  TRACEID=$(echo $output | grep 'X-B3-Traceid:' | cut -c 15-)

  run retry_default curl -s -f "localhost:16686/api/traces/$TRACEID"
}

@test "client sampling with 100% should not send traces to zipkin/jaeger" {
  run curl -s -f localhost:9993

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  TRACEID=$(echo $output | grep 'X-B3-Traceid:' | cut -c 15-)

  run retry_default curl -s -f "localhost:16686/api/traces/$TRACEID"
}

# TODO(Gufran): used for debugging, remove this after fixing listeners
@test "verify gateway listeners" {
  run curl -s -vvv localhost:9990
  echo "LISTENER 9990: $output"

  run curl -s -vvv localhost:9991
  echo "LISTENER 9991: $output"

  run curl -s -vvv localhost:9992
  echo "LISTENER 9992: $output"

  run curl -s -vvv localhost:9993
  echo "LISTENER 9993: $output"

  [ "0" == "1" ]
}