# Dnsmasq Install Script

This folder contains a script for installing [Dnsmasq](http://www.thekelleys.org.uk/dnsmasq/doc.html) and configuring 
it to forward requests for a specific domain to Consul. This way, you can easily use Consul as your DNS server for
domain names such as `foo.service.consul`, where `foo` is a service registered with Consul (see the [Registering 
Services docs](https://www.consul.io/intro/getting-started/services.html) for instructions on registering your services
with Consul). All other domain names will continue to be resolved via the default resolver on your OS. See the [Consul 
DNS Forwarding Guide](https://www.consul.io/docs/guides/forwarding.html) for more info, including trade-offs between using this module and [systemd-resolved](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/setup-systemd-resolved) for DNS forwarding.


This script has been tested on the following operating systems:

* Ubuntu 16.04
* Amazon Linux 2

There is a good chance it will work on other flavors of Debian, CentOS, and RHEL as well.



## Quick start

To install Dnsmasq, use `git` to clone this repository at a specific tag (see the [releases page](../../../../releases) 
for all available tags) and run the `install-dnsmasq` script:

```
git clone --branch <VERSION> https://github.com/hashicorp/terraform-aws-consul.git
terraform-aws-consul/modules/install-dnsmasq/install-dnsmasq
```

Note: by default, the `install-dnsmasq` script assumes that a Consul agent is already running locally and connected to 
a Consul cluster. After the install completes, restart `dnsmasq` (e.g. `sudo /etc/init.d/dnsmasq restart`) and queries 
to the `.consul` domain will be resolved via Consul:

```
dig foo.service.consul
```

We recommend running the `install-dnsmasq` script as part of a [Packer](https://www.packer.io/) template to create an
[Amazon Machine Image (AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) (see the 
[consul-ami example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/consul-ami) for sample code). 




## Command line Arguments

The `install-dnsmasq` script accepts the following arguments:

* `consul-domain DOMAIN`: The domain name to point to Consul. Optional. Default: `consul`.
* `consul-ip IP`: The IP address to use for Consul. Optional. Default: `127.0.0.1`. This assumes a Consul agent is 
  running locally and connected to a Consul cluster.
* `consul-dns-port PORT`: The port Consul uses for DNS requests. Optional. Default: `8600`.

Example:

```
install-dnsmasq
```




## Troubleshooting

Add the `+trace` argument to `dig` commands to more clearly see what's going on:

```
dig vault.service.consul +trace
```
