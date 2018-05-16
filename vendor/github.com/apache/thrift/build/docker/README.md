# Docker Integration #

Due to the large number of language requirements to build Apache Thrift, docker containers are used to build and test the project on a variety of platforms to provide maximum test coverage.

## Travis CI Integration ##

The Travis CI scripts use the following environment variables and logic to determine their behavior.

### Environment Variables ###

| Variable | Default | Usage |
| -------- | ----- | ------- |
| `DISTRO` | `ubuntu-xenial` | Set by various build jobs in `.travis.yml` to run builds in different containers.  Not intended to be set externally.|
| `DOCKER_REPO` | `thrift/thrift-build` | The name of the Docker Hub repository to obtain and store docker images. |
| `DOCKER_USER` | `<none>` | The Docker Hub account name containing the repository. |
| `DOCKER_PASS` | `<none>` | The Docker Hub account password to use when pushing new tags. |

For example, the default docker image that is used in builds if no overrides are specified would be: `thrift/thrift-build:ubuntu-xenial`

### Forks ###

If you have forked the Apache Thrift repository and you would like to use your own Docker Hub account to store thrift build images, you can use the Travis CI web interface to set the `DOCKER_USER`, `DOCKER_PASS`, and `DOCKER_REPO` variables in a secure manner.  Your fork builds will then pull, push, and tag the docker images in your account.

### Logic ###

The Travis CI build runs in two phases - first the docker images are rebuilt for each of the three supported containers if they do not match the Dockerfile that was used to build the most recent tag.  If a `DOCKER_PASS` environment variable is specified, the docker stage builds will attempt to log into Docker Hub and push the resulting tags.

## Supported Containers ##

The Travis CI (continuous integration) builds use the Ubuntu Trusty, Xenial, and Artful images to maximize language level coverage.

### Ubuntu ###

* trusty (legacy)
* xenial (stable)
* artful (latest)

## Unsupported Containers ##

These containers may be in various states, and may not build everything.

### CentOS ###
* 7.3
  * make check in lib/py may hang in test_sslsocket - root cause unknown

### Debian ###

* jessie
* stretch
  * make check in lib/cpp fails due to https://svn.boost.org/trac10/ticket/12507

## Building like Travis CI does, locally ##

We recommend you build locally the same way Travis CI does, so that when you submit your pull request you will run into fewer surprises.  To make it a little easier, put the following into your `~/.bash_aliases` file:

    # Kill all running containers.
    alias dockerkillall='docker kill $(docker ps -q)'

    # Delete all stopped containers.
    alias dockercleanc='printf "\n>>> Deleting stopped containers\n\n" && docker rm $(docker ps -a -q)'

    # Delete all untagged images.
    alias dockercleani='printf "\n>>> Deleting untagged images\n\n" && docker rmi $(docker images -q -f dangling=true)'

    # Delete all stopped containers and untagged images.
    alias dockerclean='dockercleanc || true && dockercleani'

    # Build a thrift docker image (run from top level of git repo): argument #1 is image type (ubuntu, centos, etc).
    function dockerbuild
    {
      docker build -t $1 build/docker/$1
    }

    # Run a thrift docker image: argument #1 is image type (ubuntu, centos, etc).
    function dockerrun
    {
      docker run -v $(pwd):/thrift/src -it $1 /bin/bash
    }

To pull down the current image being used to build (the same way Travis CI does it) - if it is out of date in any way it will build a new one for you:

    thrift$ DOCKER_REPO=thrift/thrift-build DISTRO=ubuntu-xenial build/docker/refresh.sh

To run all unit tests (just like Travis CI):

    thrift$ dockerrun ubuntu-xenial
    root@8caf56b0ce7b:/thrift/src# build/docker/scripts/autotools.sh

To run the cross tests (just like Travis CI):

    thrift$ dockerrun ubuntu-xenial
    root@8caf56b0ce7b:/thrift/src# build/docker/scripts/cross-test.sh

When you are done, you want to clean up occasionally so that docker isn't using lots of extra disk space:

    thrift$ dockerclean

You need to run the docker commands from the root of the git repository for them to work.

When you are done in the root docker shell you can `exit` to go back to your user host shell.  Once the unit tests and cross test passes locally, then submit he changes, and squash the pull request to one commit to make it easier to merge.  Thanks.  I am going to update the docker README.md with this information so others can leverage it too.  Now you are building like Travis CI does!

## Raw Commands for Building with Docker ##

If you do not want to use the same scripts Travis CI does, you can do it manually:

Build the image:

    thrift$ docker build -t thrift build/docker/ubuntu-xenial

Open a command prompt in the image:

    thrift$ docker run -v $(pwd):/thrift/src -it thrift /bin/bash

## Core Tool Versions per Dockerfile ##

Last updated: October 1, 2017

| Tool      | ubuntu-trusty | ubuntu-xenial | ubuntu-artful | Notes |
| :-------- | :------------ | :------------ | :------------ | :---- |
| ant       | 1.9.3         | 1.9.6         | 1.9.9         |       |
| autoconf  | 2.69          | 2.69          | 2.69          |       |
| automake  | 1.14.1        | 1.15          | 1.15          |       |
| bison     | 3.0.2         | 3.0.4         | 3.0.4         |       |
| boost     | 1.54.0        | 1.58.0        | 1.63.0        | artful: stock boost 1.62.0 has problems running unit tests |
| cmake     | 3.2.2         | 3.5.1         | 3.9.1         |       |
| cppcheck  | 1.61          | 1.72          | 1.80          |       |
| flex      | 2.5.35        | 2.6.0         | 2.6.1         |       |
| glibc     | 2.19          | 2.23          | 2.26          |       |
| libevent  | 2.0.21        | 2.0.21        | 2.1           |       |
| libstdc++ | 4.8.4         | 5.4.0         | 7.2.0         |       |
| make      | 3.81          | 4.1           | 4.1           |       |
| openssl   | 1.0.1f        | 1.0.2g        | 1.0.2g        |       |
| qt5       | 5.2.1         | 5.5.1         | 5.9.1         |       |

## Compiler/Language Versions per Dockerfile ##

Last updated: October 1, 2017

| Language  | ubuntu-trusty | ubuntu-xenial | ubuntu-artful | Notes |
| :-------- | :------------ | :------------ | :------------ | :---- |
| as3       |               |               |               | Not in CI |
| C++ gcc   | 4.8.4         | 5.4.0         | 7.2.0         |       |
| C++ clang | 3.4           | 3.8           | 4.0           |       |
| C# (mono) | 3.2.8.0       | 4.2.1         | 4.6.2.7       |       |
| c_glib    | 2.40.2        | 2.48.2        | 2.54.0        |       |
| cocoa     |               |               |               | Not in CI |
| d         | 2.070.2       | 2.073.2       | 2.076.0       |       |
| dart      | 1.20.1        | 1.24.2        | 1.24.2        |       |
| delphi    |               |               |               | Not in CI |
| dotnet    |               | 2.0.3         | 2.0.3         |       |
| erlang    | R16B03        | 18.3          | 20.0.4        |       |
| go        | 1.2.1         | 1.6.2         | 1.8.3         |       |
| haskell   | 7.6.3         | 7.10.3        | 8.0.2         |       |
| haxe      |               | 3.2.1         | 3.4.2         | disabled in trusty builds - cores on install v3.0.0, disabled in artful builds - see THRIFT-4352 |
| java      | 1.7.0_151     | 1.8.0_131     | 1.8.0_151     |       |
| js        |               |               |               | Unsure how to look for version info? |
| lua       | 5.1.5         | 5.2.4         | 5.2.4         | Lua 5.3: see THRIFT-4386 |
| nodejs    |               | 4.2.6         | 8.9.1         | trusty has node.js 0.10.0 which is too old |
| ocaml     |               | 4.02.3        | 4.04.0        |       |
| perl      | 5.18.2        | 5.22.1        | 5.26.0        |       |
| php       | 5.5.9         | 7.0.22        | 7.1.8         |       |
| python    | 2.7.6         | 2.7.12        | 2.7.14        |       |
| python3   | 3.4.3         | 3.5.2         | 3.6.3         |       |
| ruby      | 1.9.3p484     | 2.3.1p112     | 2.3.3p222     |       |
| rust      | 1.15.1        | 1.15.1        | 1.18.0        |       |
| smalltalk |               |               |               | Not in CI |
| swift     |               |               |               | Not in CI |

