# Dockerfiles for Windows Integration Tests

## Index

- [About](#about-this-file)
- [Dockerfile-fortio-windows](#dockerfile-fortio-windows)
- [Dockerfile-socat-windows](#dockerfile-socat-windows)
- [Dockerfile-bats-core-windows](#dockerfile-bats-core-windows)
- [Build images](#build-images)

## About this File

In this file you will find which Docker images that need to be pre-built to run the Envoy integration tests on Windows, as well as information on how to run each of these files individually for testing purposes.

## Dockerfile-fortio-windows

This file sole purpose is to build the custom Fortio image for Windows OS. To do this, the official [windows/nanoserver image](https://hub.docker.com/_/microsoft-windows-nanoserver) is used as base image.
To build this image you need to run the following command on your terminal:

```shell
docker build -t fortio . -f Dockerfile-fortio-windows
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```shell
docker run --rm -p 8080:8080 --name fortio fortio
```

If everything works properly you should openning the browser and check that the Fortio server running on: `http://localhost:8080/fortio`

## Dockerfile-socat-windows

The alpine:socat image was replaced by a windows core image to which a precompiled version of Socat was installed.

The windows base used was: `mcr.microsoft.com/windows/servercore:1809`

The compiled windows version of Socat can be found in the repository [https://github.com/tech128/socat-1.7.3.0-windows](https://github.com/tech128/socat-1.7.3.0-windows)

To build this image you need to run the following command on your terminal:

```shell
docker build -t socat -f Dockerfile-socat-windows .
```

You can test the built file by running the following command:

```shell
docker run --rm --name socat socat
```

If everything works properly you should get the following output:

```shell
20XX/XX/XX XX:XX:XX socat[1292] E exactly 2 addresses required (there are 0); use option "-h" for help
```

## Dockerfile-bats-core-windows

This file sole purpose is to build the custom Bats image for Windows OS. To do this, the official [windows/servercore image](https://hub.docker.com/_/microsoft-windows-servercore) is used as base image.
To build this image you need to run the following command on your terminal:

```shell
docker build -t bats-verify . -f Dockerfile-bats-windows
```

This is the same command used in run-tests.sh

You can test the built file by running the following command:

```shell
docker run --rm --name bats-verify bats-verify
```

If everything works properly you should see the help commands and available parameters about how to run Bats tests like is displayed below

```shell
$ docker run --rm --name bats-verify bats-verify
Usage: bats [OPTIONS] <tests>
       bats [-h | -v]

  <tests> is the path to a Bats test file, or the path to a directory
  containing Bats test files (ending with ".bats")
```

## Build images

To build the images, it is necessary to open a Git bash terminal and run

```
./build-images.sh
```
