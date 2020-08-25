# Consul Cluster

This folder contains a [Terraform](https://www.terraform.io/) module to deploy a
[Consul](https://www.consul.io/) cluster in [AWS](https://aws.amazon.com/) on top of an Auto Scaling Group. This module
is designed to deploy an [Amazon Machine Image (AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html)
that has Consul installed via the [install-consul](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/install-consul) module in this Module.



## How do you use this module?

This folder defines a [Terraform module](https://www.terraform.io/docs/modules/usage.html), which you can use in your
code by adding a `module` configuration and setting its `source` parameter to URL of this folder:

```hcl
module "consul_cluster" {
  # TODO: update this to the final URL
  # Use version v0.0.5 of the consul-cluster module
  source = "github.com/hashicorp/terraform-aws-consul//modules/consul-cluster?ref=v0.0.5"

  # Specify the ID of the Consul AMI. You should build this using the scripts in the install-consul module.
  ami_id = "ami-abcd1234"

  # Add this tag to each node in the cluster
  cluster_tag_key   = "consul-cluster"
  cluster_tag_value = "consul-cluster-example"

  # Configure and start Consul during boot. It will automatically form a cluster with all nodes that have that same tag.
  user_data = <<-EOF
              #!/bin/bash
              /opt/consul/bin/run-consul --server --cluster-tag-key consul-cluster --cluster-tag-value consul-cluster-example
              EOF

  # ... See variables.tf for the other parameters you must define for the consul-cluster module
}
```

Note the following parameters:

* `source`: Use this parameter to specify the URL of the consul-cluster module. The double slash (`//`) is intentional
  and required. Terraform uses it to specify subfolders within a Git repo (see [module
  sources](https://www.terraform.io/docs/modules/sources.html)). The `ref` parameter specifies a specific Git tag in
  this repo. That way, instead of using the latest version of this module from the `master` branch, which
  will change every time you run Terraform, you're using a fixed version of the repo.

* `ami_id`: Use this parameter to specify the ID of a Consul [Amazon Machine Image
  (AMI)](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) to deploy on each server in the cluster. You
  should install Consul in this AMI using the scripts in the [install-consul](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/install-consul) module.

* `user_data`: Use this parameter to specify a [User
  Data](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html#user-data-shell-scripts) script that each
  server will run during boot. This is where you can use the [run-consul script](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul) to configure and
  run Consul. The `run-consul` script is one of the scripts installed by the [install-consul](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/install-consul)
  module.

You can find the other parameters in [variables.tf](variables.tf).

Check out the [consul-cluster example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/root-example) for fully-working sample code.




## How do you connect to the Consul cluster?

### Using the HTTP API from your own computer

If you want to connect to the cluster from your own computer, the easiest way is to use the [HTTP
API](https://www.consul.io/docs/agent/http.html). Note that this only works if the Consul cluster is running in public
subnets and/or your default VPC (as in the [consul-cluster example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/root-example)), which is OK for testing
and experimentation, but NOT recommended for production usage.

To use the HTTP API, you first need to get the public IP address of one of the Consul Servers. You can find Consul
servers by using AWS tags. If you're running the [consul-cluster example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/root-example), the
[consul-examples-helper.sh script](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/consul-examples-helper/consul-examples-helper.sh) will do the tag lookup
for you automatically (note, you must have the [AWS CLI](https://aws.amazon.com/cli/),
[jq](https://stedolan.github.io/jq/), and the [Consul agent](https://www.consul.io/) installed locally):

```
> ../consul-examples-helper/consul-examples-helper.sh

Your Consul servers are running at the following IP addresses:

34.200.218.123
34.205.127.138
34.201.165.11
```

You can use one of these IP addresses with the `members` command to see a list of cluster nodes:

```
> consul members -http-addr=11.22.33.44:8500

Node                 Address             Status  Type    Build  Protocol  DC
i-0051c3ea00e9691a0  172.31.35.148:8301  alive   client  0.8.0  2         us-east-1
i-00aea529cce1761d4  172.31.47.236:8301  alive   client  0.8.0  2         us-east-1
i-01bc94ccfa032d82d  172.31.27.193:8301  alive   client  0.8.0  2         us-east-1
i-04271e97808f15d63  172.31.25.174:8301  alive   server  0.8.0  2         us-east-1
i-0483b07abe49ea7ff  172.31.5.42:8301    alive   client  0.8.0  2         us-east-1
i-098fb1ebd5ca443bf  172.31.55.203:8301  alive   client  0.8.0  2         us-east-1
i-0eb961b6825f7871c  172.31.65.9:8301    alive   client  0.8.0  2         us-east-1
i-0ee6dcf715adbff5f  172.31.67.235:8301  alive   server  0.8.0  2         us-east-1
i-0fd0e63682a94b245  172.31.54.84:8301   alive   server  0.8.0  2         us-east-1
```

You can also try inserting a value:

```
> consul kv put -http-addr=11.22.33.44:8500 foo bar

Success! Data written to: foo
```

And reading that value back:

```
> consul kv get -http-addr=11.22.33.44:8500 foo

bar
```

Finally, you can try opening up the Consul UI in your browser at the URL `http://11.22.33.44:8500/ui/`.

![Consul UI](https://github.com/hashicorp/terraform-aws-consul/blob/master/_docs/consul-ui-screenshot.png?raw=true)


### Using the Consul agent on another EC2 Instance

The easiest way to run [Consul agent](https://www.consul.io/docs/agent/basics.html) and have it connect to the Consul
cluster is to use the same EC2 tags the Consul servers use to discover each other during bootstrapping.

For example, imagine you deployed a Consul cluster in `us-east-1` as follows:

<!-- TODO: update this to the final URL -->

```hcl
module "consul_cluster" {
  source = "github.com/hashicorp/terraform-aws-consul//modules/consul-cluster?ref=v0.0.5"

  # Add this tag to each node in the cluster
  cluster_tag_key   = "consul-cluster"
  cluster_tag_value = "consul-cluster-example"

  # ... Other params omitted ...
}
```

Using the `retry-join-ec2-xxx` params, you can connect run a Consul agent on an EC2 Instance as follows:

```
consul agent -retry-join-ec2-tag-key=consul-cluster -retry-join-ec2-tag-value=consul-cluster-example -data-dir=/tmp/consul
```

Two important notes about this command:

1. By default, the Consul cluster nodes advertise their *private* IP addresses, so the command above only works from
   EC2 Instances inside the same VPC (or any VPC with proper peering connections and route table entries).
1. In order to look up the EC2 tags, the EC2 Instance where you're running this command must have an IAM role with
   the `ec2:DescribeInstances` permission.



## How do you connect load balancers to the Auto Scaling Group (ASG)?

You can use the [`aws_autoscaling_attachment`](https://www.terraform.io/docs/providers/aws/r/autoscaling_attachment.html) resource.

For example, if you are using the new application or network load balancers:

```hcl
resource "aws_lb_target_group" "test" {
  // ...
}

# Create a new Consul Cluster
module "consul" {
  source ="..."
  // ...
}

# Create a new load balancer attachment
resource "aws_autoscaling_attachment" "asg_attachment_bar" {
  autoscaling_group_name = "${module.consul.asg_name}"
  alb_target_group_arn   = "${aws_alb_target_group.test.arn}"
}
```

If you are using a "classic" load balancer:

```hcl
# Create a new load balancer
resource "aws_elb" "bar" {
  // ...
}

# Create a new Consul Cluster
module "consul" {
  source ="..."
  // ...
}

# Create a new load balancer attachment
resource "aws_autoscaling_attachment" "asg_attachment_bar" {
  autoscaling_group_name = "${module.consul.asg_name}"
  elb                    = "${aws_elb.bar.id}"
}
```



## What's included in this module?

This module creates the following architecture:

![Consul architecture](https://github.com/hashicorp/terraform-aws-consul/blob/master/_docs/architecture.png?raw=true)

This architecture consists of the following resources:

* [Auto Scaling Group](#auto-scaling-group)
* [EC2 Instance Tags](#ec2-instance-tags)
* [Security Group](#security-group)
* [IAM Role and Permissions](#iam-role-and-permissions)


### Auto Scaling Group

This module runs Consul on top of an [Auto Scaling Group (ASG)](https://aws.amazon.com/autoscaling/). Typically, you
should run the ASG with 3 or 5 EC2 Instances spread across multiple [Availability
Zones](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html). Each of the EC2
Instances should be running an AMI that has Consul installed via the [install-consul](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/install-consul)
module. You pass in the ID of the AMI to run using the `ami_id` input parameter.


### EC2 Instance Tags

This module allows you to specify a tag to add to each EC2 instance in the ASG. We recommend using this tag with the
[retry_join_ec2](https://www.consul.io/docs/agent/options.html?#retry_join_ec2) configuration to allow the EC2
Instances to find each other and automatically form a cluster.


### Security Group

Each EC2 Instance in the ASG has a Security Group that allows:

* All outbound requests
* All the inbound ports specified in the [Consul documentation](https://www.consul.io/docs/agent/options.html?#ports-used)

The Security Group ID is exported as an output variable if you need to add additional rules.

Check out the [Security section](#security) for more details.


### IAM Role and Permissions

Each EC2 Instance in the ASG has an [IAM Role](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html) attached.
We give this IAM role a small set of IAM permissions that each EC2 Instance can use to automatically discover the other
Instances in its ASG and form a cluster with them. See the [run-consul required permissions
docs](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul#required-permissions) for details.

The IAM Role ARN is exported as an output variable if you need to add additional permissions.

You can disable the creation of the IAM role and policies if needed by setting `enable_iam_setup` variable to false.  This allows you to create the role seperately from this module and supply the external role arn via the `iam_instance_profile_name` variable.


## How do you roll out updates?

If you want to deploy a new version of Consul across the cluster, the best way to do that is to:

1. Build a new AMI.
1. Set the `ami_id` parameter to the ID of the new AMI.
1. Run `terraform apply`.

This updates the Launch Configuration of the ASG, so any new Instances in the ASG will have your new AMI, but it does
NOT actually deploy those new instances. To make that happen, you should do the following:

1. Issue an API call to one of the old Instances in the ASG to have it leave gracefully. E.g.:

    ```
    curl -X PUT <OLD_INSTANCE_IP>:8500/v1/agent/leave
    ```

1. Once the instance has left the cluster, terminate it:

    ```
    aws ec2 terminate-instances --instance-ids <OLD_INSTANCE_ID>
    ```

1. After a minute or two, the ASG should automatically launch a new Instance, with the new AMI, to replace the old one.

1. Wait for the new Instance to boot and join the cluster.

1. Repeat these steps for each of the other old Instances in the ASG.

We will add a script in the future to automate this process (PRs are welcome!).




## What happens if a node crashes?

There are two ways a Consul node may go down:

1. The Consul process may crash. In that case, `systemd` should restart it automatically.
1. The EC2 Instance running Consul dies. In that case, the Auto Scaling Group should launch a replacement automatically.
   Note that in this case, since the Consul agent did not exit gracefully, and the replacement will have a different ID,
   you may have to manually clean out the old nodes using the [force-leave
   command](https://www.consul.io/docs/commands/force-leave.html). We may add a script to do this
   automatically in the future. For more info, see the [Consul Outage
   documentation](https://www.consul.io/docs/guides/outage.html).




## Security

Here are some of the main security considerations to keep in mind when using this module:

1. [Encryption in transit](#encryption-in-transit)
1. [Encryption at rest](#encryption-at-rest)
1. [Dedicated instances](#dedicated-instances)
1. [Security groups](#security-groups)
1. [SSH access](#ssh-access)


### Encryption in transit

Consul can encrypt all of its network traffic. For instructions on enabling network encryption, have a look at the
[How do you handle encryption documentation](https://github.com/hashicorp/terraform-aws-consul/tree/master/modules/run-consul#how-do-you-handle-encryption).


### Encryption at rest

The EC2 Instances in the cluster store all their data on the root EBS Volume. To enable encryption for the data at
rest, you must enable encryption in your Consul AMI. If you're creating the AMI using Packer (e.g. as shown in
the [consul-ami example](https://github.com/hashicorp/terraform-aws-consul/tree/master/examples/consul-ami)), you need to set the [encrypt_boot
parameter](https://www.packer.io/docs/builders/amazon-ebs.html#encrypt_boot) to `true`.


### Dedicated instances

If you wish to use dedicated instances, you can set the `tenancy` parameter to `"dedicated"` in this module.


### Security groups

This module attaches a security group to each EC2 Instance that allows inbound requests as follows:

* **Consul**: For all the [ports used by Consul](https://www.consul.io/docs/agent/options.html#ports), you can
  use the `allowed_inbound_cidr_blocks` parameter to control the list of
  [CIDR blocks](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing) that will be allowed access and the `allowed_inbound_security_group_ids` parameter to control the security groups that will be allowed access.

* **SSH**: For the SSH port (default: 22), you can use the `allowed_ssh_cidr_blocks` parameter to control the list of
  [CIDR blocks](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing) that will be allowed access. You can use the `allowed_inbound_ssh_security_group_ids` parameter to control the list of source Security Groups that will be allowed access.

Note that all the ports mentioned above are configurable via the `xxx_port` variables (e.g. `server_rpc_port`). See
[variables.tf](variables.tf) for the full list.



### SSH access

You can associate an [EC2 Key Pair](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html) with each
of the EC2 Instances in this cluster by specifying the Key Pair's name in the `ssh_key_name` variable. If you don't
want to associate a Key Pair with these servers, set `ssh_key_name` to an empty string.





## What's NOT included in this module?

This module does NOT handle the following items, which you may want to provide on your own:

* [Monitoring, alerting, log aggregation](#monitoring-alerting-log-aggregation)
* [VPCs, subnets, route tables](#vpcs-subnets-route-tables)
* [DNS entries](#dns-entries)


### Monitoring, alerting, log aggregation

This module does not include anything for monitoring, alerting, or log aggregation. All ASGs and EC2 Instances come
with limited [CloudWatch](https://aws.amazon.com/cloudwatch/) metrics built-in, but beyond that, you will have to
provide your own solutions.


### VPCs, subnets, route tables

This module assumes you've already created your network topology (VPC, subnets, route tables, etc). You will need to
pass in the the relevant info about your network topology (e.g. `vpc_id`, `subnet_ids`) as input variables to this
module.


### DNS entries

This module does not create any DNS entries for Consul (e.g. in Route 53).


