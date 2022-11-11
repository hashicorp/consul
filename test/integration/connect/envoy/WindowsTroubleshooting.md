# Envoy Integration Tests on Windows

## Index

- [About this Guide](#about-this-guide)
- [Prerequisites](#prerequisites)
- [Running the Tests](#running-the-tests)
- [Troubleshooting](#troubleshooting)
  - [About Envoy Integration Tests on Windows](#about-envoy-integration-tests-on-windows)
  - [Common Errors](#common-errors)
- [Windows Scripts Changes](#windows-scripts-changes)
- [Volume Issues](#volume-issues)  

## About this Guide

On this guide you will find all the information required to run the Envoy integration tests on Windows.

## Prerequisites

To run the integration tests yo will need to have the following installed on your System:

- GO v1.18(or later).
- Gotestsum library [installation](https://pkg.go.dev/gotest.tools/gotestsum).
- Docker.

Before running the tests, you will need to build the required Docker images, to do so, you can use the script provided [here](../../../../build-support-windows/build-images.sh):

- Build Images Script Execution
  - From a Bash console (GitBash or WSL) execute: `./build-images.sh`

## Running the Tests

To execute the tests you need to run the following command depending on the shell you are using:  
**On Powershell**:  
`go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/<TEST CASE>" -win=true`  
Where **TEST CASE** is the individual test case we want to execute (e.g. case-badauthz).  

**On Git Bash**:  
`ENVOY_VERSION=<ENVOY VERSION> go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/<TEST CASE>" -win=true`  
Where **TEST CASE** is the individual test case we want to execute (e.g. case-badauthz), and **ENVOY VERSION** is the version which you are currently testing.

> [!TIP]
> When executing the integration tests using **Powershell** you may need to set the ENVOY_VERSION value manually in line 20 of the [run-tests.windows.sh](run-tests.windows.sh) file.

> [!WARNING]
> When executing the integration tests for Windows environments, the **End of Line Sequence** of every related file and/or script will be changed from **LF** to **CRLF**.

### About Envoy Integration Tests on Windows

Integration tests on Linux run a multi-container architecture that take advantage of the Host Network Docker feature, using this feature means that the container's network stack is not isolated from the Docker host (the container shares the host’s networking namespace), and the container does not get its own IP-address allocated (read more about this [here](https://docs.docker.com/network/host/)). This feature is only available for Linux, which made migrating the tests to Windows challenging, since replicating the same architecture created more issues, that's why a **single container** architecture was chosen to run the Envoy integration tests.  
Using a single container architecture meant that we could use the same tests as on linux, moreover we were able to speed-up their execution by replacing *docker run* commands which started utility containers, for *docker exec* commands.

### Common errors

If the tests are executed without docker running, the following error will be seen:

```powershell
error during connect: This error may indicate that the docker daemon is not running.: Post "http://%2F%2F.%2Fpipe%2Fdocker_engine/v1.24/build?buildargs=%7B%7D&cachefrom=%5B%5D&cgroupparent=&cpuperiod=0&cpuquota=0&cpusetcpus=&cpusetmems=&cpushares=0&dockerfile=Dockerfile-bats-windows&labels=%7B%7D&memory=0&memswap=0&networkmode=default&rm=1&shmsize=0&t=bats-verify&target=&ulimits=null&version=1": open //./pipe/docker_engine: The system cannot find the file specified.
```

If any of the docker images does not exist or is mistagged, an error similar to the following will be displayed:

```powershell
Error response from daemon: No such container: envoy_workdir_1
```

If you run the Windows tests from WSL you will get the following error message:

```bash
main_test.go:34: command failed: exec: "cmd": executable file not found in $PATH
```

## Windows Scripts Changes

- The  "http-addr", "grpc-addr" and "admin-access-log-path" flags were added to the creation of the Envoy Bootstrap files.
- To execute commands sh was replaced by bash on our Windows container.
- All paths were updated to use Windows format.
- Created *stop_and_copy_files* function to copy files into the shared volume (see [volume issues](#volume-issues)).
- Changed the *-admin-bind* value from `0.0.0.0` to `127.0.0.1` when generating the Envoy Bootstrap files.
- Removed the *&&* from the *common_run_container_service's* docker exec command and replaced it with *\*.
- Removed *docker_wget* and *docker_curl* functions from [helpers.windows.bash](helpers.windows.bash) file and replaced them with **docker_consul_exec**, this way we avoid starting intermediate containers when capturing logs.
- The function *wipe_volumes* uses a `docker exec` command instead of the original `docker run`, this way we speed up test execution by avoiding to start a new container just to delete volume content before each test run.
- For **case-grpc** we increased the `envoy_stats_flush_interval` value from 1s to 5s, on Windows, the original value caused the test to pass or fail randomly.
- For **case-wanfed-gw** a new script was created: **global-setup-windows.sh**, this file replaces global-setup.sh when running this test in Windows. The new script uses the windows/consul:local Docker image to generate the required TLS files and copies them into host's workdir directory.
- To use the **debug_dump_volumes** function, you need to use it via Powershell and execute the following command: `bash run-tests.windows.sh debug_dump_volumes` Make sure to be positioned with your terminal in the correct directory.
- For **case-consul-exec** this case can only be run when using the consul-dev Docker image on this repository, since it relies on features implemented only here. These features are: Windows valid default value for "-admin-access-log-path" and `consul connect envoy` command starts Envoy. This features have also been submitted in [PR#15114](https://github.com/hashicorp/consul/pull/15114).  

## Volume Issues

Another difference that arose when migrating the tests from Linux to Windows, is that file system operations can't be executed while Windows containers are running. Currently, when running the tests a **named volume** is created and all of the required files are copied into that volume. Because of the constraint mentioned before, the workaround we implemented was creating a function (**stop_and_copy_files**) that stops the *kubernetes/pause* container and executes a script to copy the required files and finally starts the container again.
