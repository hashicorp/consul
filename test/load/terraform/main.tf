terraform {
  required_version = ">= 0.12"
}

data "aws_ami" "consul" {
  most_recent = true

  owners = var.ami_owners

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

# ---------------------------------------------------------------------------------------------------------------------
# Create variables and ssh keys
# ---------------------------------------------------------------------------------------------------------------------


variable "name" {
  description = "Used to name various infrastructure components"
  default     = "consul-test"
}
resource "random_pet" "test" {
}

locals {
  random_name = "${var.name}-${random_pet.test.id}"
}

module "keys" {
  name    = local.random_name
  path    = "${path.root}/keys"
  source  = "mitchellh/dynamic-keys/aws"
  version = "v2.0.0"
}


# ---------------------------------------------------------------------------------------------------------------------
# Create VPC with public and also private subnets
# ---------------------------------------------------------------------------------------------------------------------

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "2.21.0"

  name               = var.vpc_name
  cidr               = var.vpc_cidr
  azs                = var.vpc_az
  public_subnets     = var.public_subnet_cidrs
  private_subnets    = var.private_subnet_cidrs
  enable_nat_gateway = true
}

# ---------------------------------------------------------------------------------------------------------------------
# Deploy consul cluster
# ---------------------------------------------------------------------------------------------------------------------

module "consul" {
  source       = "hashicorp/consul/aws"
  version      = "0.7.9"
  depends_on   = [module.vpc.vpc_id]
  ami_id       = var.ami_id
  ssh_key_name = module.keys.key_name
  vpc_id       = module.vpc.vpc_id
  cluster_name = var.cluster_name
  num_clients  = var.num_clients
  num_servers  = var.num_servers
}

# ---------------------------------------------------------------------------------------------------------------------
# This script will configure and start Consul agents
# ---------------------------------------------------------------------------------------------------------------------

data "template_file" "user_data_server" {
  template = file("${path.module}/user-data-server.sh")

  vars = {
    cluster_tag_key   = var.cluster_tag_key
    cluster_tag_value = var.cluster_name
  }
}

data "template_file" "user_data_client" {
  template = file("${path.module}/user-data-client.sh")

  vars = {
    cluster_tag_key   = var.cluster_tag_key
    cluster_tag_value = var.cluster_name
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# Start up test servers to run tests from
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_default_security_group" "loadtest" {
  vpc_id = module.vpc.vpc_id

  ingress {
    protocol  = 6
    self      = true
    from_port = 22
    to_port   = 22
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    protocol = 6
    self = true
    from_port = 8500
    to_port = 8500
  }
}

resource "aws_launch_configuration" "test-servers" {
  name_prefix = "${var.cluster_name}-test-"
  image_id      = var.test_server_ami
  instance_type = var.test_instance_type
  key_name      = module.keys.key_name
  security_groups = [aws_default_security_group.loadtest.id]

  associate_public_ip_address = var.test_public_ip
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "test-servers" {
  name                      = aws_launch_configuration.test-servers.name
  launch_configuration      = aws_launch_configuration.test-servers.id
  min_size                  = 0
  max_size                  = 5
  desired_capacity          = 2
  wait_for_capacity_timeout = "480s"
  health_check_grace_period = 15
  health_check_type         = "EC2"
  vpc_zone_identifier       = tolist([module.vpc.public_subnets[0], module.vpc.private_subnets[0], module.vpc.private_subnets[1]])

  lifecycle {
    create_before_destroy = true
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

  vpc_id             = module.vpc.vpc_id
  subnets            = module.vpc.private_subnets
  security_groups    = [module.consul.security_group_id_clients]
  internal = true

  target_groups = [
    {
      #name_prefix has a six char limit
      name_prefix      = "test-"
      backend_protocol = "HTTP"
      backend_port     = 8500
      target_type      = "instance"
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
  autoscaling_group_name = module.consul.asg_name_clients
  alb_target_group_arn   = module.alb.target_group_arns[0]
}
