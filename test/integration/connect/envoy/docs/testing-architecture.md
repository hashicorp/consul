# Testing Architecture

## Linux Test Architecture

On Linux, tests take advantage of the Host network feature (only available for Linux containers). This means that every container within the network shares the host’s networking namespace. The network stack for every container that uses this network mode won’t be isolated from the Docker host and won’t get their own IP address.  

![linux-architecture](./img/linux-arch.png)

Every time a test is run, a directory called workdir is created, here all the required files to run the tests are copied. Then this same directory is mounted as a **named volume**, a container with a Kubernetes pause image tagged as *envoy_workdir_1* is run to keep the volume accessible as other containers start while running the tests. Linux containers allow file system operations on runtime unlike Windows containers.  

## Current Windows Architecture

As we previously mentioned, on Windows there is no Host networking feature, so we went with NAT network instead. The main consequences of this is that now each container has their own networking stack (IP address) separated from each other, they can communicate among themselves using Docker's DNS feature (using the containers name) but no longer through localhost.  
Another problem we are facing while sticking to this architecture, is that configuration files assume that every service (services run by fortio and Envoy's sidecar proxy service) are running in localhost. Though we had some partial success on modifying those files on runtime still we are finding issues related to this.
Test's assertions are composed of either function calls or curl executions, we managed this by mapping those calls to the corresponding container name.

![windows-architecture-current](./img/windows-arch-current.png)

Above, the failing connections are depicted. We kept the same architecture as on Linux and worked around trying to solve those connectivity issues.
