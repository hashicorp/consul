# Docker Consul Demo

A consul cluster demo in a number of Ubuntu-based containers

To get started, build the image:

    $ make image

Then, to bring up a cluster:

    $ make cluster

Once it is finished, you should be able to list the cluster members with _docker ps_.
All nodes expose their ports to loopback addresses, the web-ui and dns are exposed on their default ports (80 and 53).
All nodes in the default cluster share the configuration in the dc1 directory via a docker volume.

Attaching to one of the containers should be easier in docker 1.3 with:

    $ docker exec server1 /bin/bash

Cleanup can be done with make clean and make erase (the latter implies clean and also removes the image)

To learn more about starting Consul, joining nodes and interacting with the agent,
checkout the [getting started guide](http://www.consul.io/intro/getting-started/install.html).

