# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

data "aws_ami" "consul" {
  most_recent = true

  owners = var.ami_owners

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "is-public"
    values = ["false"]
  }

  filter {
    name   = "name"
    values = ["consul-ubuntu-*"]
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# Deploy consul cluster
# ---------------------------------------------------------------------------------------------------------------------

module "consul_servers" {
  source = "git::git@github.com:hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=v0.8.0"

  cluster_name      = "${var.cluster_name}-server"
  cluster_size      = var.num_servers
  instance_type     = var.instance_type
  cluster_tag_key   = var.cluster_tag_key
  cluster_tag_value = var.cluster_name

  ami_id    = var.consul_ami_id == null ? data.aws_ami.consul.id : var.consul_ami_id
  user_data = data.template_file.user_data_server.rendered

  vpc_id                  = module.vpc.vpc_id
  subnet_ids              = module.vpc.public_subnets
  allowed_ssh_cidr_blocks = [var.vpc_cidr]

  allowed_inbound_cidr_blocks = [var.vpc_cidr]
  ssh_key_name                = module.keys.key_name

}

module "consul_clients" {
  source            = "git::git@github.com:hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=v0.8.0"
  cluster_name      = "${var.cluster_name}-client"
  cluster_size      = var.num_clients
  instance_type     = var.instance_type
  cluster_tag_key   = var.cluster_tag_key
  cluster_tag_value = var.cluster_name

  ami_id    = var.consul_ami_id == null ? data.aws_ami.consul.id : var.consul_ami_id
  user_data = data.template_file.user_data_client.rendered

  vpc_id                  = module.vpc.vpc_id
  subnet_ids              = module.vpc.public_subnets
  allowed_ssh_cidr_blocks = [var.vpc_cidr]

  allowed_inbound_cidr_blocks = [var.vpc_cidr]
  ssh_key_name                = module.keys.key_name
}


# ---------------------------------------------------------------------------------------------------------------------
# This script will configure and start Consul agents
# ---------------------------------------------------------------------------------------------------------------------

data "template_file" "user_data_server" {
  template = file("${path.module}/user-data-server.sh")

  vars = {
    consul_version      = var.consul_version
    consul_download_url = var.consul_download_url
    cluster_tag_key     = var.cluster_tag_key
    cluster_tag_value   = var.cluster_name
  }
}

data "template_file" "user_data_client" {
  template = file("${path.module}/user-data-client.sh")

  vars = {
    consul_version      = var.consul_version
    consul_download_url = var.consul_download_url
    cluster_tag_key     = var.cluster_tag_key
    cluster_tag_value   = var.cluster_name
  }
}

# 
#  Set up ALB for test-servers to talk to consul clients
# 
module "alb" {

  source  = "terraform-aws-modules/alb/aws"
  version = "~> 5.0"

  name = "${var.cluster_name}-alb"

  load_balancer_type = "application"

  vpc_id          = module.vpc.vpc_id
  subnets         = module.vpc.public_subnets
  security_groups = [module.consul_clients.security_group_id]
  internal        = true

  target_groups = [
    {
      #name_prefix has a six char limit
      name_prefix      = "test-"
      backend_protocol = "HTTP"
      backend_port     = 8500
      target_type      = "instance"
      health_check = {
        interval          = 5
        timeout           = 3
        protocol          = "HTTP"
        healthy_threshold = 2
        path              = "/v1/status/leader"
      }
    }
  ]

  http_tcp_listeners = [
    {
      port               = 8500
      protocol           = "HTTP"
      target_group_index = 0
    }
  ]
}

# Attach ALB to Consul clients
resource "aws_autoscaling_attachment" "asg_attachment_bar" {
  autoscaling_group_name = module.consul_clients.asg_name
  alb_target_group_arn   = module.alb.target_group_arns[0]
}
