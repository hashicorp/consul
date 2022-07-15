#!/usr/bin/env bats

load dummy-function

setup() {
  source vars.sh
  source setup.sh "Content of the created setup.txt file in setup.sh" $TXT_FILE_NAME
}

teardown() {
  cat /dev/null >$TXT_FILE_NAME
}

@test "Test with dummyFunction invoked" {
  FIRST_ARG="First Argument"

  run dummyFunction "$FIRST_ARG"

  [ $status -eq 0 ]
  [ -n "$output" ] # Not empty
  [ "$output" = "$FIRST_ARG $COMMON_VAR" ]
}

@test "Test skipped" {
  skip

  run not_existing_function

  [ "$status" -eq 100000 ]
}

@test "Test Function with Curl" {
  run curlFunction

  [ $status -eq 0 ]
  [ -n "$output" ]
  [ "$output" = "200" ]
}

@test "Test Function with jq" {
  local INPUT='{"key1": "Test Value 1", "key2": "Test Value 2"}'
  run jqFunction "$INPUT" "key1"

  [ $status -eq 0 ]
  [ -n "$output" ]
  [ "$output" = "\"Test Value 1\"" ]
}