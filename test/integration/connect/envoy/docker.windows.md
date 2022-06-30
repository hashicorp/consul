# Docker Files for Windows Integration Tests

## Index

- [About](#about-this-file)
- [Dockerfile-test-sds-server-windows](#dockerfile-test-sds-server-windows)

## About this File

In this file you will find which Dockerfiles are needed to run the Envoy integration tests on Windows, as well as information on how to run each of these files individually for testing purposes.

## Dockerfile-test-sds-server-windows

This file sole purpose is to build the test-sds-server executable using Go. To do so, we use an official [golang image](https://hub.docker.com/_/golang/) provided in docker hub with Windows nano server.
To build this image you need to run the following command on your terminal:

`docker build -t test-sds-server -f Dockerfile-test-sds-server-windows test-sds-server`

This is the same command used in run-tests.sh

You can test the built file by running the following command:

`docker run --rm -p 1234:1234 --name test-sds-server test-sds-server:latest`

If everything works properly you should get the following output:

```Powershell
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=ca-root
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=foo.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=wildcard.ingress.consul
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  Loaded cert from file: name=www.example.com
20XX-XX-XXTXX:XX:XX.XXX-XXX [INFO]  ==> SDS listening: addr=0.0.0.0:1234
```
