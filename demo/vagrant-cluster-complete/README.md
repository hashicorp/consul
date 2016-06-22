# README #

This demo creates a small Consul cluster using Vagrant and Consul
configuration files.  The cluster is intended to be equal to that
given in the [Consul
documentation](https://www.consul.io/intro/getting-started/join.html).

To get started, you can start the cluster by just doing:

    $ vagrant up

Once it is finished, you should be able to see the following:

    $ vagrant status
    Current machine states:
    n1                        running (virtualbox)
    n2                        running (virtualbox)

At this point the two nodes are running and you can SSH in to play
with them and run the commands shown in the documentation, e.g.:

    $vagrant ssh n1
    vagrant@n1:~$ consul members
    vagrant@n1:~$ dig @127.0.0.1 -p 8600 agent-two.node.consul
    vagrant@n1:~$ curl http://localhost:8500/v1/health/state/critical
    ... etc.

The [Key/Value
Data](https://www.consul.io/intro/getting-started/kv.html)
demonstration is scripted in key_value.sh.  Run it as follows:

    $vagrant ssh n1
    vagrant@n1:~$ /vagrant/key_value.sh
    
You can also see the Consul UI at http://localhost:8501/ui/#/dc1/services.

To learn more about starting Consul, joining nodes and interacting with the agent,
checkout the [getting started guide](http://www.consul.io/intro/getting-started/install.html).


## Notes

* `Vagrantfile` uses `provision.sh` to provision two nodes, `n1` and `n2`.
* Files used during configuration of both nodes are in `common`,
  node-specific files for configuration and health checks are in their
  subfolders.
* `consul.conf` is for the consul service running on each machine.
* `key_value.sh` is a short script to populate the key/values for the cluster.


## Differences from the Consul documentation and existing demo.

* This demo uses `hashicorp/precise64` instead of `debian/wheezy64`
  for the Vagrant box.
* `provision.sh` uses upstart script instead of `consul agent`.
  Originally, `provision.sh` used `exec consul agent
  -config-file=/etc/consul.d/config.json &` to start consul in a
  background thread so the vagrantfile could run both machines to
  completion.  This would start consul correctly, but it didn't appear
  to be stable: the consul nodes would shut down after vagrant ssh'ing
  into a node, leaving (using either exit or logout), and then vagrant
  ssh'ing back into the same node.
* n2/config.json's 0.0.0.0 client_addr, and the forwarded port in
  Vagrantfile, is to allow the UI to be visible from the host
  machine's browser as localhost, and also to allow for consul method
  calls to be made in the n2 vm without specifying the RPC (remote
  proc call) address "-rpc-addr".
* Two web services, "healthy" and "unhealthy", are defined for cluster
  n2.  The
  [Services](https://www.consul.io/intro/getting-started/services.html)
  page assumes that a healthy web service is available, while the
  [Health
  Checks](https://www.consul.io/intro/getting-started/checks.html)
  page assumes an unhealthy service is available.