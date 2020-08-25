# Consul Install Script

This folder contains a script for installing Consul and its dependencies. Use this script along with the
[run-consul script](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul) to create a Consul [Amazon Machine Image 
(AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) that can be deployed in 
[AWS](https://aws.amazon.com/) across an Auto Scaling Group using the [consul-cluster module](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/consul-cluster).

This script has been tested on the following operating systems:

* Ubuntu 16.04
* Ubuntu 18.04
* Amazon Linux 2

There is a good chance it will work on other flavors of Debian, CentOS, and RHEL as well.



## Quick start

<!-- TODO: update the clone URL to the final URL when this Module is released -->

To install Consul, use `git` to clone this repository at a specific tag (see the [releases page](../../../../releases) 
for all available tags) and run the `install-consul` script:

```
git clone --branch <VERSION> https://github.com/hashicorp/terraform-aws-consul.git
terraform-aws-consul/modules/install-consul/install-consul --version 0.8.0
```

The `install-consul` script will install Consul, its dependencies, and the [run-consul script](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul).
The `run-consul` script is also run when the server is booting to start Consul and configure it to automatically 
join other nodes to form a cluster.

We recommend running the `install-consul` script as part of a [Packer](https://www.packer.io/) template to create a
Consul [Amazon Machine Image (AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) (see the 
[consul-ami example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/consul-ami) for a fully-working sample code). You can then deploy the AMI across an Auto 
Scaling Group using the [consul-cluster module](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/consul-cluster) (see the [consul-cluster 
example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/root-example) for fully-working sample code).




## Command line Arguments

The `install-consul` script accepts the following arguments:

* `version VERSION`: Install Consul version VERSION. Optional if download-url is provided.
* `download-url URL`: Install the Consul package hosted in this url. Optional if version is provided.
* `path DIR`: Install Consul into folder DIR. Optional.
* `user USER`: The install dirs will be owned by user USER. Optional.
* `ca-file-path PATH`: Path to a PEM-encoded certificate authority used to encrypt and verify authenticity of client and server connections. Optional.
* `cert-file-path PATH`: Path to a PEM-encoded certificate, which will be provided to clients or servers to verify the agent's authenticity. Optional.
* `key-file-path PATH`: Path to a PEM-encoded private key, used with the certificate to verify the agent's authenticity. Optional.

Example:

```
install-consul --version 1.2.2
```



## How it works

The `install-consul` script does the following:

1. [Creates a user and folders for Consul](#create-a-user-and-folders-for-consul)
1. [Installs Consul binaries and scripts](#install-consul-binaries-and-scripts)
1. [Installs provided TLS certificates](#install-tls-certificates)
1. [Follow-up tasks](#follow-up-tasks)


### Creates a user and folders for Consul

Creates an OS user named `consul`. Creates the following folders, all owned by user `consul`:

* `/opt/consul`: base directory for Consul data (configurable via the `--path` argument).
* `/opt/consul/bin`: directory for Consul binaries.
* `/opt/consul/data`: directory where the Consul agent can store state.
* `/opt/consul/config`: directory where the Consul agent looks up configuration.
* `/opt/consul/log`: directory where Consul will store log output.
* `/opt/consul/tls`: directory where an optional server certificate and private key are copied if provided.
* `/opt/consul/tls/ca`: directory where an optional CA certificate is copied if provided.


### Installs Consul binaries and scripts

Installs the following:

* `consul`: Either downloads the Consul zip file from the [downloads page](https://www.consul.io/downloads.html) (the version
  number is configurable via the `--version` argument), or a package hosted on a precise url configurable with `--dowload-url`
  (useful for installing Consul Enterprise, for example) and extracts the `consul` binary into `/opt/consul/bin`. Adds a
  symlink to the `consul` binary in `/usr/local/bin`.
* `run-consul`: Copies the [run-consul script](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul) into `/opt/consul/bin`.

### Installs TLS certificates

Copies the certificates/key provided by the `--ca-file-path`, `cert-file-path` and `key-file-path` to the Consul
configuration directory. If provided, the CA file is copied to `/opt/consul/tls/ca` and the server certificate/key
are copied to `/opt/consul/tls` (assuming the default config path of `/opt/consul`). The script also sets the
required permissions and file ownership.

### Follow-up tasks

After the `install-consul` script finishes running, you may wish to do the following:

1. If you have custom Consul config (`.json`) files, you may want to copy them into the config directory (default:
   `/opt/consul/config`).
1. If `/usr/local/bin` isn't already part of `PATH`, you should add it so you can run the `consul` command without
   specifying the full path.



## Dependencies

The install script assumes that `systemd` is already installed.  We use it as a cross-platform supervisor to ensure Consul is started
whenever the system boots and restarted if the Consul process crashes.  Additionally, it is used to store all logs which can be accessed
using `journalctl`.



## Why use Git to install this code?

We needed an easy way to install these scripts that satisfied a number of requirements, including working on a variety 
of operating systems and supported versioning. Our current solution is to use `git`, but this may change in the future.
See [Package Managers](https://github.com/hashicorp/terraform-aws-consul/tree/master/_docs/package-managers.md) for a full discussion of the requirements, trade-offs, and why we
picked `git`.
