# Envoy Integration Tests on Windows

## Index

- [About](#about)
- [Pre-built core images](#pre-built-core-images)
- [Test images](#integration-test-images)
- [Run Tests](#run-tests)

## About

This file is the entrypoint to understand how to execute Envoy integration tests on Windows as well as to understand the differences between Linux tests and Windows tests. Below you can find a list of relevant documentation that has been written while working on supporting the Envoy integration tests on Windows.

- [Windows Testing Architecture](test/integration/connect/envoy/docs/windows-testing-architecture.md): On this file you will find why the testing architecture on Windows differs from Linux's.
- [Build Images](build-support-windows/BUILD-IMAGES.md): Here you will find how to build the images required for executing the tests.
- [Windows Troubleshooting](test/integration/connect/envoy/WindowsTroubleshooting.md): This file lists, among other things everything we needed to change/adapt for the existing tests to run in Windows containers.

## Pre-built core images

Before running the integration tests, you must pre-build the core images that the tests require to be ran on the Windows environment. Make sure to check out the `BUILD-IMAGES` file [here](build-support-windows/BUILD-IMAGES.md) for this purpose.

## Integration test images

During the execution of the integration tests, several images are built based-on the pre-built core images. To get more information about these and how to run them independently, please check out the `docker.windows` file [here](test/integration/connect/envoy/docker.windows.md).

## Run tests

To run all the integration tests, you need to execute next command

```shell
go test -v -timeout=30s -tags integration ./test/integration/connect/envoy -run="TestEnvoy" -win=true
```

To run a single test case, the name should be specified. For instance, to run the `case-badauthz` test, you need to execute next command

```shell
go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-badauthz" -win=true
```

> :warning: Note that the flag `-win=true` must be specified as shown in the above commands. This flag is very important because the same allows to indicate that the tests will be executed on the Windows environment. When executing the Envoy integration tests the **End of Line Sequence** of every related file and or script will be automatically changed from **LF to CRLF**.  
