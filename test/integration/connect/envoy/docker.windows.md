# Docker Files for Windows Integration Tests

## Index

- [About](#about-this-file)
- [Pre-requisites](#pre-requisites)
- [Dockerfile-bats-windows](#dockerfile-bats-windows)
- [Dockerfile-test-sds-server-windows](#dockerfile-test-sds-server-windows)

## About this File

In this file you will find which Dockerfiles are needed to run the Envoy integration tests on Windows, as well as information on how to run each of these files individually for testing purposes.

## Pre-requisites
After building and running the images and containers, you need to have pre-built the base images used by these Dockerfiles. See [pre-built images required in Windows](../../../../build-support-windows/docker.windows.md)

## Dockerfile-bats-windows

This file sole purpose is to build the custom Bats image for Windows OS. To do this, the official [windows/servercore image](https://hub.docker.com/_/microsoft-windows-servercore) is used as base image.
To build this image you need to run the following command on your terminal:

```shell
docker build -t bats-verify . -f Dockerfile-bats-windows
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```shell
docker run --rm --name bats-verify -v $(pwd -W)/case-dummy-bats:C:\\workdir bats-verify --pretty *.bats
```

If everything works properly you should see the result of the dummy test executed as is displayed below

```shell
docker run --rm --name bats-verify -v $(pwd -W)/case-dummy-bats:C:\\workdir bats-verify --pretty *.bats
verify_1.bats
    ✔ Basic Test 1
    ✔ Basic Test 2
verify_2.bats
    ✔ Test with dummyFunction invoked
    - Test skipped (skipped)

4 tests, 0 failures, 1 skipped
```

## Dockerfile-test-sds-server-windows

This file sole purpose is to build the test-sds-server executable using Go. To do so, we use an official [golang image](https://hub.docker.com/_/golang/) provided in docker hub with Windows nano server.
To build this image you need to run the following command on your terminal:

```shell
docker build -t test-sds-server -f Dockerfile-test-sds-server-windows test-sds-server
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```shell
docker run --rm -p 1234:1234 --name test-sds-server test-sds-server
```

If everything works properly you should get the following output:

```shell
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=ca-root
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=foo.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=wildcard.ingress.consul
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=www.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  ==> SDS listening: addr=0.0.0.0:1234
```
