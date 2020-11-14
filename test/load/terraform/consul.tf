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

module "consul" {
  source       = "hashicorp/consul/aws"
  version      = "0.7.9"
  depends_on   = [module.vpc.vpc_id]
  ami_id       = var.consul_ami_id
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

# 
#  Set up ALB for test-servers to talk to consul clients
# 
module "alb" {

  source  = "terraform-aws-modules/alb/aws"
  version = "~> 5.0"

  name = "${var.cluster_name}-${local.random_name}-alb"

  load_balancer_type = "application"

  vpc_id          = module.vpc.vpc_id
  subnets         = module.vpc.public_subnets
  security_groups = [module.consul.security_group_id_clients]
  internal        = true

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
