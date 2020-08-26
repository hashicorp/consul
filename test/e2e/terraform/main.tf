# ---------------------------------------------------------------------------------------------------------------------
# DEPLOY A CONSUL CLUSTER IN AWS
# These templates show an example of how to use the consul-cluster module to deploy Consul in AWS. We deploy two Auto
# Scaling Groups (ASGs): one with a small number of Consul server nodes and one with a larger number of Consul client
# nodes. Note that these templates assume that the AMI you provide via the ami_id input variable is built from
# the examples/consul-ami/consul.json Packer template.
# ---------------------------------------------------------------------------------------------------------------------

# ----------------------------------------------------------------------------------------------------------------------
# REQUIRE A SPECIFIC TERRAFORM VERSION OR HIGHER
# This module has been updated with 0.12 syntax, which means it is no longer compatible with any versions below 0.12.
# ----------------------------------------------------------------------------------------------------------------------
terraform {
  required_version = ">= 0.12"
}

# ---------------------------------------------------------------------------------------------------------------------
# AUTOMATICALLY LOOK UP THE LATEST PRE-BUILT AMI
# This repo contains a CircleCI job that automatically builds and publishes the latest AMI by building the Packer
# template at /examples/consul-ami upon every new release. The Terraform data source below automatically looks up the
# latest AMI so that a simple "terraform apply" will just work without the user needing to manually build an AMI and
# fill in the right value.
#
# !! WARNING !! These exmaple AMIs are meant only convenience when initially testing this repo. Do NOT use these example
# AMIs in a production setting because it is important that you consciously think through the configuration you want
# in your own production AMI.
#
# NOTE: This Terraform data source must return at least one AMI result or the entire template will fail. See
# /_ci/publish-amis-in-new-account.md for more information.
# ---------------------------------------------------------------------------------------------------------------------
data "aws_ami" "consul" {
  most_recent = true

  # If we change the AWS Account in which test are run, update this value.
  owners = ["562637147889"]

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "is-public"
    values = ["true"]
  }

  filter {
    name   = "name"
    values = ["consul-ubuntu-*"]
  }
}

## Fix up da keys
variable "name" {
  description = "Used to name various infrastructure components"
  default     = "consul-e2e"
}
resource "random_pet" "e2e" {
}

locals {
  random_name = "${var.name}-${random_pet.e2e.id}"
}

# First generate key pair for EC2 instances
module "keys" {
  name    = local.random_name
  path    = "${path.root}/keys"
  source  = "mitchellh/dynamic-keys/aws"
  version = "v2.0.0"
}

#Create VPC with public of private subnets

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "2.21.0"

  name               = var.vpc_name
  cidr               = var.vpc_cidr
  azs                = var.vpc_az
  public_subnets     = var.public_subnet_cidr
  enable_nat_gateway = true
}

# ---------------------------------------------------------------------------------------------------------------------
# DEPLOY THE CONSUL SERVER NODES
# ---------------------------------------------------------------------------------------------------------------------

module "consul_servers" {
  # When using these modules in your own templates, you will need to use a Git URL with a ref attribute that pins you
  # to a specific version of the modules, such as the following example:
  # source = "git::git@github.com:hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=v0.0.1"
  source        = "./modules/consul-cluster"
  depends_on    = [module.vpc.vpc_id]
  cluster_name  = "${var.cluster_name}-server"
  cluster_size  = var.num_servers
  instance_type = "t2.micro"
  spot_price    = var.spot_price

  # The EC2 Instances will use these tags to automatically discover each other and form a cluster
  cluster_tag_key   = var.cluster_tag_key
  cluster_tag_value = var.cluster_name

  ami_id    = "${var.ami_id == null ? data.aws_ami.consul.image_id : var.ami_id}"
  user_data = "${data.template_file.user_data_server.rendered}"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.public_subnets

  # To make testing easier, we allow Consul and SSH requests from any IP address here but in a production
  # deployment, we strongly recommend you limit this to the IP address ranges of known, trusted servers inside your VPC.
  allowed_ssh_cidr_blocks = ["0.0.0.0/0"]

  allowed_inbound_cidr_blocks = ["0.0.0.0/0"]
  ssh_key_name                = module.keys.key_name

  tags = [
    {
      key                 = "Environment"
      value               = "development"
      propagate_at_launch = true
    }
  ]
}

# ---------------------------------------------------------------------------------------------------------------------
# THE USER DATA SCRIPT THAT WILL RUN ON EACH CONSUL SERVER EC2 INSTANCE WHEN IT'S BOOTING
# This script will configure and start Consul
# ---------------------------------------------------------------------------------------------------------------------

data "template_file" "user_data_server" {
  template = file("${path.module}/user-data-server.sh")

  vars = {
    cluster_tag_key   = var.cluster_tag_key
    cluster_tag_value = var.cluster_name
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# DEPLOY THE CONSUL CLIENT NODES
# Note that you do not have to use the consul-cluster module to deploy your clients. We do so simply because it
# provides a convenient way to deploy an Auto Scaling Group with the necessary IAM and security group permissions for
# Consul, but feel free to deploy those clients however you choose (e.g. a single EC2 Instance, a Docker cluster, etc).
# ---------------------------------------------------------------------------------------------------------------------

module "consul_clients" {
  # When using these modules in your own templates, you will need to use a Git URL with a ref attribute that pins you
  # to a specific version of the modules, such as the following example:
  # source = "git::git@github.com:hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=v0.0.1"
  source        = "./modules/consul-cluster"
  depends_on    = [module.vpc.vpc_id]
  cluster_name  = "${var.cluster_name}-client"
  cluster_size  = var.num_clients
  instance_type = "t2.micro"
  spot_price    = var.spot_price

  cluster_tag_key   = "consul-clients"
  cluster_tag_value = var.cluster_name

  ami_id    = "${var.ami_id == null ? data.aws_ami.consul.image_id : var.ami_id}"
  user_data = "${data.template_file.user_data_client.rendered}"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.public_subnets

  # To make testing easier, we allow Consul and SSH requests from any IP address here but in a production
  # deployment, we strongly recommend you limit this to the IP address ranges of known, trusted servers inside your VPC.
  allowed_ssh_cidr_blocks = ["0.0.0.0/0"]

  allowed_inbound_cidr_blocks = ["0.0.0.0/0"]
  ssh_key_name                = module.keys.key_name
}

# ---------------------------------------------------------------------------------------------------------------------
# THE USER DATA SCRIPT THAT WILL RUN ON EACH CONSUL CLIENT EC2 INSTANCE WHEN IT'S BOOTING
# This script will configure and start Consul
# ---------------------------------------------------------------------------------------------------------------------

data "template_file" "user_data_client" {
  template = file("${path.module}/user-data-client.sh")

  vars = {
    cluster_tag_key   = var.cluster_tag_key
    cluster_tag_value = var.cluster_name
  }
}




# creates testing-servers autoscaling group for testing
resource "aws_autoscaling_group" "test-servers" {
  name                      = aws_launch_configuration.test-servers.name
  launch_configuration      = aws_launch_configuration.test-servers.name
  min_size                  = 0
  max_size                  = 5
  desired_capacity          = 2
  wait_for_capacity_timeout = "480s"
  health_check_grace_period = 15
  health_check_type         = "EC2"
  vpc_zone_identifier       = [tolist(module.vpc.public_subnets)[0]]

  lifecycle {
    create_before_destroy = true
  }
}

# provides a resource for a new autoscaling group launch configuration
resource "aws_launch_configuration" "test-servers" {
  name            = "test-servers-"
  image_id        = var.testserverami
  instance_type   = var.testsize
  key_name        = module.keys.key_name
  security_groups = ["${module.consul_servers.security_group_id}"]

  associate_public_ip_address = var.asg_public_ip

  lifecycle {
    create_before_destroy = true
  }
}


# ---------------------------------------------------------------------------------------------------------------------
# DEPLOY CONSUL IN THE DEFAULT VPC AND SUBNETS
# Using the default VPC and subnets makes this example easy to run and test, but it means Consul is accessible from the
# public Internet. For a production deployment, we strongly recommend deploying into a custom VPC with private subnets.
# ---------------------------------------------------------------------------------------------------------------------

data "aws_vpc" "default" {
  default = var.vpc_id == null ? true : false
  id      = "${var.vpc_id}"
}

data "aws_subnet_ids" "default" {
  vpc_id = data.aws_vpc.default.id
}

data "aws_region" "current" {
}


