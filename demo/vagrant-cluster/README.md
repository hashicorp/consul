# Vagrant Consul Demo

This demo provides a very simple Vagrantfile that creates two nodes,
one at "172.20.20.10" and another at "172.20.20.11". Both are running
a standard Debian 7 distribution, and Consul is pre-installed.

To get started, you can start the cluster by just doing:

    $ vagrant up

Once it is finished, you should be able to see the following:

    $ vagrant status
    Current machine states:
    n1                        running (vmware_fusion)
    n2                        running (vmware_fusion)

At this point the two nodes are running and you can SSH in to play with them:

    $ vagrant ssh n1
    ...
    $ vagrant ssh n2
    ...

To learn more about starting Consul, joining nodes and interacting with the agent,
checkout the [getting started guide](https://www.consul.io/intro/getting-started/install.html).

