# Envoy Integration Tests

## Overview

These tests validate that Consul is configuring Envoy correctly. They set up various scenarios using Docker containers and then run
[Bats](https://github.com/sstephenson/bats) (a Bash test framework) tests to validate the expected results.

## Running Tests

To run the tests locally, `cd` into the root of the repo and run:

```console
make test-envoy-integ
```

To run a specific test, run:

```console
make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/case-basic"
```

Where `case-basic` can be replaced by any directory name from this directory.

## How Do These Tests Work

1. The tests are all run through Go test via the `main_test.go` file. Each directory prefixed by `case-` is a subtest, for example,
`TestEnvoy/case-basic` and `TestEnvoy/case-wanfed-gw`.
2. The real framework for this test suite lives in `run-tests.sh`. Under the hood, `main_test.go` just runs `run-tests.sh` with
   various arguments.
3. The tests use your local code by building a Docker image from your local directory just before executing. 
4. The tests run Docker containers connected by a shared Docker network. All tests have at least one Consul server running and then
   depending on the test case they will spin up additional services or gateways. Some tests run multiple Consul servers to test
   multi-DC setups.
5. At a high level, tests are set up by executing the `setup.sh` script in each directory. This script uses helper functions
   defined in `helpers.bash`. Once the test case is set up, the validations in `verify.bats` are run.
6. In CI, the tests are executed against different Envoy versions and with both `XDS_TARGET=client` and `XDS_TARGET=server`.
   If set to `client`, a Consul server and client are run, and services are registered against the client. If set to `server`,
   only a Consul server is run, and services are registered against the server. By default, `XDS_TARGET` is set to `server`.
   See [this comment](https://github.com/hashicorp/consul/blob/70bb6a2abdbc5ed4a6e728e8da243c5394a631d1/test/integration/connect/envoy/run-tests.sh#L178-L212) for more information.

## Investigating Test Failures

* When tests fail in CI, logs and additional debugging data are available in the artifacts of the test run.
* You can re-run the tests locally by running `make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/<case-directory>"` where `<case-directory>` is
  replaced with the name of the directory, e.g. `case-basic`.
* Locally, all the logs of the failed test will be available in `workdir` in this directory.
* You can run with `DEBUG=1` to print out all the commands being run, e.g. `DEBUG=1 make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/case-basic"`.
* If you want to prevent the Docker containers from being spun down after test failure, add a `sleep 9999` to the `verify.bats` test case that's failing.