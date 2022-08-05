
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



