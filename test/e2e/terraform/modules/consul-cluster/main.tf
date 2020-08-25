# ----------------------------------------------------------------------------------------------------------------------
# REQUIRE A SPECIFIC TERRAFORM VERSION OR HIGHER
# This module has been updated with 0.12 syntax, which means it is no longer compatible with any versions below 0.12.
# ----------------------------------------------------------------------------------------------------------------------

terraform {
  required_version = ">= 0.12"
}

# ---------------------------------------------------------------------------------------------------------------------
# CREATE AN AUTO SCALING GROUP (ASG) TO RUN CONSUL
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_autoscaling_group" "autoscaling_group" {
  name_prefix = var.cluster_name

  launch_configuration = aws_launch_configuration.launch_configuration.name

  availability_zones  = var.availability_zones
  vpc_zone_identifier = var.subnet_ids

  # Run a fixed number of instances in the ASG
  min_size             = var.cluster_size
  max_size             = var.cluster_size
  desired_capacity     = var.cluster_size
  termination_policies = [var.termination_policies]

  health_check_type         = var.health_check_type
  health_check_grace_period = var.health_check_grace_period
  wait_for_capacity_timeout = var.wait_for_capacity_timeout
  service_linked_role_arn   = var.service_linked_role_arn

  enabled_metrics = var.enabled_metrics

  protect_from_scale_in = var.protect_from_scale_in

  tags = flatten(
    [
      {
        key                 = "Name"
        value               = var.cluster_name
        propagate_at_launch = true
      },
      {
        key                 = var.cluster_tag_key
        value               = var.cluster_tag_value
        propagate_at_launch = true
      },
      var.tags,
    ]
  )
}

# ---------------------------------------------------------------------------------------------------------------------
# CREATE LAUNCH CONFIGURATION TO DEFINE WHAT RUNS ON EACH INSTANCE IN THE ASG
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_launch_configuration" "launch_configuration" {
  name_prefix   = "${var.cluster_name}-"
  image_id      = var.ami_id
  instance_type = var.instance_type
  user_data     = var.user_data
  spot_price    = var.spot_price

  iam_instance_profile = var.enable_iam_setup ? element(
    concat(aws_iam_instance_profile.instance_profile.*.name, [""]),
    0,
  ) : var.iam_instance_profile_name
  key_name = var.ssh_key_name

  security_groups = concat(
    [aws_security_group.lc_security_group.id],
    var.additional_security_group_ids,
  )
  placement_tenancy           = var.tenancy
  associate_public_ip_address = var.associate_public_ip_address

  ebs_optimized = var.root_volume_ebs_optimized

  root_block_device {
    volume_type           = var.root_volume_type
    volume_size           = var.root_volume_size
    delete_on_termination = var.root_volume_delete_on_termination
  }

  # Important note: whenever using a launch configuration with an auto scaling group, you must set
  # create_before_destroy = true. However, as soon as you set create_before_destroy = true in one resource, you must
  # also set it in every resource that it depends on, or you'll get an error about cyclic dependencies (especially when
  # removing resources). For more info, see:
  #
  # https://www.terraform.io/docs/providers/aws/r/launch_configuration.html
  # https://terraform.io/docs/configuration/resources.html
  lifecycle {
    create_before_destroy = true
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# CREATE A SECURITY GROUP TO CONTROL WHAT REQUESTS CAN GO IN AND OUT OF EACH EC2 INSTANCE
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_security_group" "lc_security_group" {
  name_prefix = var.cluster_name
  description = "Security group for the ${var.cluster_name} launch configuration"
  vpc_id      = var.vpc_id

  # aws_launch_configuration.launch_configuration in this module sets create_before_destroy to true, which means
  # everything it depends on, including this resource, must set it as well, or you'll get cyclic dependency errors
  # when you try to do a terraform destroy.
  lifecycle {
    create_before_destroy = true
  }

  tags = merge(
    {
      "Name" = var.cluster_name
    },
    var.security_group_tags,
  )
}

resource "aws_security_group_rule" "allow_ssh_inbound" {
  count       = length(var.allowed_ssh_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.ssh_port
  to_port     = var.ssh_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_ssh_cidr_blocks

  security_group_id = aws_security_group.lc_security_group.id
}

resource "aws_security_group_rule" "allow_ssh_inbound_from_security_group_ids" {
  count                    = var.allowed_ssh_security_group_count
  type                     = "ingress"
  from_port                = var.ssh_port
  to_port                  = var.ssh_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_ssh_security_group_ids, count.index)

  security_group_id = aws_security_group.lc_security_group.id
}

resource "aws_security_group_rule" "allow_all_outbound" {
  type        = "egress"
  from_port   = 0
  to_port     = 0
  protocol    = "-1"
  cidr_blocks = ["0.0.0.0/0"]

  security_group_id = aws_security_group.lc_security_group.id
}

# ---------------------------------------------------------------------------------------------------------------------
# THE CONSUL-SPECIFIC INBOUND/OUTBOUND RULES COME FROM THE CONSUL-SECURITY-GROUP-RULES MODULE
# ---------------------------------------------------------------------------------------------------------------------

module "security_group_rules" {
  source = "../consul-security-group-rules"

  security_group_id                    = aws_security_group.lc_security_group.id
  allowed_inbound_cidr_blocks          = var.allowed_inbound_cidr_blocks
  allowed_inbound_security_group_ids   = var.allowed_inbound_security_group_ids
  allowed_inbound_security_group_count = var.allowed_inbound_security_group_count

  server_rpc_port = var.server_rpc_port
  cli_rpc_port    = var.cli_rpc_port
  serf_lan_port   = var.serf_lan_port
  serf_wan_port   = var.serf_wan_port
  http_api_port   = var.http_api_port
  dns_port        = var.dns_port
}

# ---------------------------------------------------------------------------------------------------------------------
# ATTACH AN IAM ROLE TO EACH EC2 INSTANCE
# We can use the IAM role to grant the instance IAM permissions so we can use the AWS CLI without having to figure out
# how to get our secret AWS access keys onto the box.
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_iam_instance_profile" "instance_profile" {
  count = var.enable_iam_setup ? 1 : 0

  name_prefix = var.cluster_name
  path        = var.instance_profile_path
  role        = element(concat(aws_iam_role.instance_role.*.name, [""]), 0)

  # aws_launch_configuration.launch_configuration in this module sets create_before_destroy to true, which means
  # everything it depends on, including this resource, must set it as well, or you'll get cyclic dependency errors
  # when you try to do a terraform destroy.
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_iam_role" "instance_role" {
  count = var.enable_iam_setup ? 1 : 0

  name_prefix        = var.cluster_name
  assume_role_policy = data.aws_iam_policy_document.instance_role.json

  # aws_iam_instance_profile.instance_profile in this module sets create_before_destroy to true, which means
  # everything it depends on, including this resource, must set it as well, or you'll get cyclic dependency errors
  # when you try to do a terraform destroy.
  lifecycle {
    create_before_destroy = true
  }
}

data "aws_iam_policy_document" "instance_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# THE IAM POLICIES COME FROM THE CONSUL-IAM-POLICIES MODULE
# ---------------------------------------------------------------------------------------------------------------------

module "iam_policies" {
  source = "../consul-iam-policies"

  enabled     = var.enable_iam_setup
  iam_role_id = element(concat(aws_iam_role.instance_role.*.id, [""]), 0)
}

