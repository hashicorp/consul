
# Windows operation

## Steps for Windows operation

- GO installation
- Library installation
- Build Images Execution
  - From a Bash console execute: `./build-images.sh`
- Execution of the tests
  - It is important to execute the CMD or Powershell tests


### Common errors

If the tests are executed without docker running, the following error will be seen:
```shell
error during connect: This error may indicate that the docker daemon is not running.: Post "http://%2F%2F.%2Fpipe%2Fdocker_engine/v1.24/build?buildargs=%7B%7D&cachefrom=%5B%5D&cgroupparent=&cpuperiod=0&cpuquota=0&cpusetcpus=&cpusetmems=&cpushares=0&dockerfile=Dockerfile-bats-windows&labels=%7B%7D&memory=0&memswap=0&networkmode=default&rm=1&shmsize=0&t=bats-verify&target=&ulimits=null&version=1": open //./pipe/docker_engine: The system cannot find the file specified.
```

If any of the docker images does not exist or is mistagged, an error similar to the following will be displayed:
```powershell
Error response from daemon: No such container: envoy_workdir_1
```

If you run the Windows tests from WSL you will get the following error message:
```powershell
main_test.go:34: command failed: exec: "cmd": executable file not found in $PATH
```

## Considerations on differences in scripts

- Creation of a new directory test case that includes the basic Windows configuration files. These configuration files include the definition of "local_service_address".
- The  "http-addr", "grpc-addr" and "admin-access-log-path" flags were added to the creation of the Envoy Bootstrap files.
- The so called "sh" were changed for "Bash" calls in Windows containers.
- The creation of a function that recovers the IP of a docker container mounted.
- The IP address of the consultation is used in the "setup_upsert_l4_intention" function.
- The "config-dir" path of the creation of the images of "envoy_consul" was adapted to adapt to the Windows format.
- The use of the function "stop_and_copy_files" was included after the creation of the bootstrap files to include these among the shared files in the volume.

## Difference over network types

There are fundamental differences in networking between the Linux version and the Windows version. In Linux, a Host type network is used that links all the containers through localhost, but this type of network does not exist in Windows. In Windows, the use of a NAT-type network was chosen. This difference is the cause of many of the problems that exist when running the tests on Windows since, in Host-type networks, any call to localhost from any of the containers would refer to the Docker host. This brings problems in two different categories, on the one hand when it comes to setting up the containers required for the test environment and on the other hand when running the tests themselves.

### Differences when lifting containers

When building the test environment in the current architecture running with Windows, we find that there are problems linking the different containers with Consul. Many default settings are used in the Linux scheme. This assumes that the services are running on the same machine, so it checks pointing to "localhost". But, in windows architecture these configurations don't work since each container is considered an independent entity with its own localhost. In this aspect, the registration of the services in consul had to be modified so that they included the address of the sidecar, since without it the connection to the services is not made.

```powershell
services {
  connect {
    sidecar_service {
      proxy {
        local_service_address = "s1-sidecar-proxy"
      }
    }
  }
}
```

### Differences in test calls

The tests are carried out from the **envoy_verify-primary_1** bats container, in all cases pointing to localhost to verify some feature. When pointing to localhost, within the windows network it takes it as if it were pointing to itself and for that reason they fail. To solve it, a function was created that maps each port with a hostname and from there locates the assigned IP and returns the corresponding IP and port.

```powershell
@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

ADDRESS=$(nslookup envoy_s1-sidecar-proxy_1)
CONTAINER_HOSTPORT="${HOSTPORT/127.0.0.1:19000/"${ADDRESS}:19000"}"
```

## Problems with the Workdir

A problem was found with the method set for creating the volume. The way the volume is currently created in Windows creates a static volume. This means that every time you want to reflect a change in the containers, it must be deleted and recreated. For this reason, every time a file is required to be modified from outside the application, the **stop_and_copy_files** function must be executed.
