# systemd-resolved Setup Script

This folder contains a script for configuring [systemd-resolved](http://man7.org/linux/man-pages/man8/systemd-resolved.service.8.html) 
to forward requests for a specific domain to Consul. This way, you can easily use Consul as your DNS server for
domain names such as `foo.service.consul`, where `foo` is a service registered with Consul (see the [Registering 
Services docs](https://www.consul.io/intro/getting-started/services.html) for instructions on registering your services
with Consul). All other domain names will continue to be resolved via the default resolver on your OS. See the [Consul 
DNS Forwarding Guide](https://www.consul.io/docs/guides/forwarding.html) and [Github Issue](https://github.com/hashicorp/consul/issues/4155) for more info.


This script has been tested on the following operating systems:

* Ubuntu 18.04

## Quick start

To setup systemd-resolved, use `git` to clone this repository at a specific tag (see the [releases page](../../../../releases) 
for all available tags) and run the `setup-systemd-resolved` script:

```
git clone --branch <VERSION> https://github.com/hashicorp/terraform-aws-consul.git
terraform-aws-consul/modules/setup-systemd-resolved/setup-systemd-resolved
```

Note: by default, the `setup-systemd-resolved` script assumes that a Consul agent is already running locally and connected to 
a Consul cluster. After the install completes, restart `systemd-resolved` (e.g. `sudo systemctl restart systemd-resolved.service`) and queries 
to the `.consul` domain will be resolved via Consul:

```
dig foo.service.consul
```

We recommend running the `setup-systemd-resolved` script as part of a [Packer](https://www.packer.io/) template to create an
[Amazon Machine Image (AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) (see the 
[consul-ami example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/consul-ami) for sample code). 




## Command line Arguments

The `setup-systemd-resolved` script accepts the following arguments:

* `consul-domain DOMAIN`: The domain name to point to Consul. Optional. Default: `consul`.
* `consul-ip IP`: The IP address to use for Consul. Optional. Default: `127.0.0.1`. This assumes a Consul agent is 
  running locally and connected to a Consul cluster.
* `consul-dns-port PORT`: The port Consul uses for DNS requests. Optional. Default: `8600`.

Example:

```
setup-systemd-resolved
```




## Troubleshooting

Add the `+trace` argument to `dig` commands to more clearly see what's going on:

```
dig vault.service.consul +trace
```
