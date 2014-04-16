# Vagrant + Docker Consul Demo

This demo provides a simple Vagrantfile that launches multiple Docker containers inside a single VM. All containers are running a standard Ubuntu 12.04 distribution, and Consul is installed and configured.

To get started, you can start the cluster by just doing:

    $ vagrant up

Once it is finished, you should be able to see the following:

    $ vagrant status
    Current machine states:
    default                   running (vmware_fusion)

At this point the VM is running and you can check the running containers

    $ vagrant ssh
    $ TODO: add examples here

To learn more about starting Consul, joining nodes and interacting with the agent,
checkout the [getting started guide](http://www.consul.io/intro/getting-started/install.html).

