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
   _Note:_ this is implemented as the `docker-envoy-integ` Makefile target which is a prerequisite to the `test-envoy-integ` target,
   so if you are running the tests by invoking `run-tests.sh` or `go test` manually, be sure to rebuild the Docker image to ensure
   you are running your latest code.
4. The tests run Docker containers connected by a shared Docker network. All tests have at least one Consul server running and then
   depending on the test case they will spin up additional services or gateways. Some tests run multiple Consul servers to test
   multi-DC setups. See the [`case-wanfed-gateway` test](./case-wanfed-gw) for an example of this.
5. At a high level, tests are set up by executing the `setup.sh` script in each directory. This script uses helper functions
   defined in `helpers.bash`. Once the test case is set up, the validations in `verify.bats` are run.
6. If there exists a `vars.sh` file in the top-level of the case directory, the test runner will source it prior to invoking
   the `run_tests`, `test_teardown` and `capture_logs` phases of the test scenario.
7. If there exists a `capture.sh` file in the top-level of the case directory, it will be executed after the test is done, but prior to
   the containers being removed. This is useful for capturing logs or Envoy snapshots for debugging test failures.
8. Any files matching the `*.hcl` glob will be copied to the container `$WORKDIR/$CLUSTER/consul` directory prior to running the tests.
   This is useful for defining Consul configuration for each agent process to load on start up.
9. In CI, the tests are executed against different Envoy versions and with both `XDS_TARGET=client` and `XDS_TARGET=server`.
   If set to `client`, a Consul server and client are run, and services are registered against the client. If set to `server`,
   only a Consul server is run, and services are registered against the server. By default, `XDS_TARGET` is set to `server`.
   See [this comment](https://github.com/hashicorp/consul/blob/70bb6a2abdbc5ed4a6e728e8da243c5394a631d1/test/integration/connect/envoy/run-tests.sh#L178-L212) for more information.

## Investigating Test Failures

* When tests fail in CI, logs and additional debugging data are available in the artifacts of the test run.
* You can re-run the tests locally by running `make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/<case-directory>"` where `<case-directory>` is
  replaced with the name of the directory, e.g. `case-basic`.
* You can override the envoy version by specifying `ENVOY_VERSION=<your envoy version>` eg. `ENVOY_VERSION=1.27.0 make test-envoy-integ`.
* Locally, all the logs of the failed test will be available in `workdir` in this directory.
* You can run with `DEBUG=1` to print out all the commands being run, e.g. `DEBUG=1 make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/case-basic"`.
* If you want to prevent the Docker containers from being spun down after test failure, add a `sleep 9999` to the `verify.bats` test case that's failing.

## Creating a New Test

Below is a rough outline for creating a new test. For the example, assume our test case will be called `my-feature`.
1. Create a new directory named `case-my-feature`
2. If the test involves multiple datacenters/clusters, create a separate subdirectory for each cluster (eg. `case-my-feature/{dc1,dc2}`)
3. Add any necessary configuration to `*.hcl` files in the respective cluster subdirectory (or the test case directory when using a single cluster).
4. Create a `setup.sh` file in the case directory
5. Create a `capture.sh` file in the case directory
6. Create a `verify.bats` file in the case directory
7. Populate the `setup.sh`, `capture.sh` and `verify.bats` files with the appropriate code for running your test, validating its state and capturing any logs or snapshots.
