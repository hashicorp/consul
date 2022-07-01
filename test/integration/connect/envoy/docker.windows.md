# Docker Files for Windows Integration Tests

## Index

- [About](#about-this-file)
- [Dockerfile-test-sds-server-windows](#dockerfile-test-sds-server-windows)
- [Dockerfile-fortio-windows](#dockerfile-fortio-windows)
- [Dockerfile-socat-windows](#dockerfile-socat-windows)

## About this File

In this file you will find which Dockerfiles are needed to run the Envoy integration tests on Windows, as well as information on how to run each of these files individually for testing purposes.

## Dockerfile-test-sds-server-windows

This file sole purpose is to build the test-sds-server executable using Go. To do so, we use an official [golang image](https://hub.docker.com/_/golang/) provided in docker hub with Windows nano server.
To build this image you need to run the following command on your terminal:

```Powershell
docker build -t test-sds-server -f Dockerfile-test-sds-server-windows test-sds-server
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```Powershell
docker run --rm -p 1234:1234 --name test-sds-server test-sds-server
```

If everything works properly you should get the following output:

```Powershell
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=ca-root
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=foo.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=wildcard.ingress.consul
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=www.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  ==> SDS listening: addr=0.0.0.0:1234
```

## Dockerfile-fortio-windows

This file sole purpose is to build the custom Fortio image for Windows OS. To do this, the official [windows/nanoserver image](https://hub.docker.com/_/microsoft-windows-nanoserver) is used as base image.
To build this image you need to run the following command on your terminal:

```Powershell
docker build -t fortio . -f Dockerfile-fortio-windows
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```Powershell
docker run --rm -p 8080:8080 --name fortio fortio
```

If everything works properly you should openning the browser and check that the Fortio server running on: `http://localhost:8080/fortio`

## Dockerfile-socat-windows

The alpine:socat image was replaced by a windows core image to which a precompiled version of Socat was installed.

The windows base used was: `mcr.microsoft.com/windows/servercore:1809`

The compiled windows version of Socat can be found in the repository [https://github.com/tech128/socat-1.7.3.0-windows](https://github.com/tech128/socat-1.7.3.0-windows)

To build this image you need to run the following command on your terminal:

```Powershell
docker build -t socat -f Dockerfile-socat-windows .
```

You can test the built file by running the following command:

```Powershell
docker run --rm --name socat socat
```

If everything works properly you should get the following output:

```Powershell
20XX/XX/XX XX:XX:XX socat[1292] E exactly 2 addresses required (there are 0); use option "-h" for help
```
