## ---------------------------------------------------------------------------------------------------------------------
# THESE TEMPLATES REQUIRE TERRAFORM VERSION 0.12 AND ABOVE
# ---------------------------------------------------------------------------------------------------------------------

terraform {
  required_version = ">= 0.12"
}

## ---------------------------------------------------------------------------------------------------------------------
# CREATE THE SECURITY GROUP RULES THAT CONTROL WHAT TRAFFIC CAN GO IN AND OUT OF A CONSUL AGENT CLUSTER
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_security_group_rule" "allow_serf_lan_tcp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.serf_lan_port
  to_port     = var.serf_lan_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_lan_udp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.serf_lan_port
  to_port     = var.serf_lan_port
  protocol    = "udp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_lan_tcp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.serf_lan_port
  to_port                  = var.serf_lan_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_lan_udp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.serf_lan_port
  to_port                  = var.serf_lan_port
  protocol                 = "udp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

# Similar to the *_inbound_from_security_group_ids rules, allow inbound from ourself

resource "aws_security_group_rule" "allow_serf_lan_tcp_inbound_from_self" {
  type      = "ingress"
  from_port = var.serf_lan_port
  to_port   = var.serf_lan_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_lan_udp_inbound_from_self" {
  type      = "ingress"
  from_port = var.serf_lan_port
  to_port   = var.serf_lan_port
  protocol  = "udp"
  self      = true

  security_group_id = var.security_group_id
}

