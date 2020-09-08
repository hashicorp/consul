# Consul AMI

## Quick start

To build the Consul AMI:

1. `git clone` this repo to your computer.
1. Install [Packer](https://www.packer.io/).
1. Configure your AWS credentials using one of the [options supported by the AWS
   SDK](http://docs.aws.amazon.com/sdk-for-java/v1/developer-guide/credentials.html). Usually, the easiest option is to
   set the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables.
1. Update the `variables` section of the `consul.json` Packer template to configure the AWS region, Consul version, and
   Dnsmasq version you wish to use. If you want to install Consul Enterprise, skip the version variable and instead set 
   the `download_url` to the full url that points to the consul enterprise zipped package.
1. Run `packer build consul.json`.

When the build finishes, it will output the IDs of the new AMIs. To see how to deploy one of these AMIs, check out the
[consul-cluster example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/root-example).
