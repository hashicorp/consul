# ----------------------------------------------------------------------------------------------------------------------
# REQUIRE A SPECIFIC TERRAFORM VERSION OR HIGHER
# This module has been updated with 0.12 syntax, which means it is no longer compatible with any versions below 0.12.
# ----------------------------------------------------------------------------------------------------------------------
terraform {
  required_version = ">= 0.12"
}

# ---------------------------------------------------------------------------------------------------------------------
# CREATE THE SECURITY GROUP RULES THAT CONTROL WHAT TRAFFIC CAN GO IN AND OUT OF A CONSUL CLUSTER
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_security_group_rule" "allow_server_rpc_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.server_rpc_port
  to_port     = var.server_rpc_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_cli_rpc_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.cli_rpc_port
  to_port     = var.cli_rpc_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_tcp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.serf_wan_port
  to_port     = var.serf_wan_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_udp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.serf_wan_port
  to_port     = var.serf_wan_port
  protocol    = "udp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_http_api_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.http_api_port
  to_port     = var.http_api_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_tcp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.dns_port
  to_port     = var.dns_port
  protocol    = "tcp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_udp_inbound" {
  count       = length(var.allowed_inbound_cidr_blocks) >= 1 ? 1 : 0
  type        = "ingress"
  from_port   = var.dns_port
  to_port     = var.dns_port
  protocol    = "udp"
  cidr_blocks = var.allowed_inbound_cidr_blocks

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_server_rpc_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.server_rpc_port
  to_port                  = var.server_rpc_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_cli_rpc_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.cli_rpc_port
  to_port                  = var.cli_rpc_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_tcp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.serf_wan_port
  to_port                  = var.serf_wan_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_udp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.serf_wan_port
  to_port                  = var.serf_wan_port
  protocol                 = "udp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_http_api_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.http_api_port
  to_port                  = var.http_api_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_tcp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.dns_port
  to_port                  = var.dns_port
  protocol                 = "tcp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_udp_inbound_from_security_group_ids" {
  count                    = var.allowed_inbound_security_group_count
  type                     = "ingress"
  from_port                = var.dns_port
  to_port                  = var.dns_port
  protocol                 = "udp"
  source_security_group_id = element(var.allowed_inbound_security_group_ids, count.index)

  security_group_id = var.security_group_id
}

# Similar to the *_inbound_from_security_group_ids rules, allow inbound from ourself

resource "aws_security_group_rule" "allow_server_rpc_inbound_from_self" {
  type      = "ingress"
  from_port = var.server_rpc_port
  to_port   = var.server_rpc_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_cli_rpc_inbound_from_self" {
  type      = "ingress"
  from_port = var.cli_rpc_port
  to_port   = var.cli_rpc_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_tcp_inbound_from_self" {
  type      = "ingress"
  from_port = var.serf_wan_port
  to_port   = var.serf_wan_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_serf_wan_udp_inbound_from_self" {
  type      = "ingress"
  from_port = var.serf_wan_port
  to_port   = var.serf_wan_port
  protocol  = "udp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_http_api_inbound_from_self" {
  type      = "ingress"
  from_port = var.http_api_port
  to_port   = var.http_api_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_tcp_inbound_from_self" {
  type      = "ingress"
  from_port = var.dns_port
  to_port   = var.dns_port
  protocol  = "tcp"
  self      = true

  security_group_id = var.security_group_id
}

resource "aws_security_group_rule" "allow_dns_udp_inbound_from_self" {
  type      = "ingress"
  from_port = var.dns_port
  to_port   = var.dns_port
  protocol  = "udp"
  self      = true

  security_group_id = var.security_group_id
}

# ---------------------------------------------------------------------------------------------------------------------
# THE CONSUL-CLIENT SPECIFIC INBOUND/OUTBOUND RULES COME FROM THE CONSUL-CLIENT-SECURITY-GROUP-RULES MODULE
# ---------------------------------------------------------------------------------------------------------------------

module "client_security_group_rules" {
  source = "../consul-client-security-group-rules"

  security_group_id                    = var.security_group_id
  allowed_inbound_cidr_blocks          = var.allowed_inbound_cidr_blocks
  allowed_inbound_security_group_ids   = var.allowed_inbound_security_group_ids
  allowed_inbound_security_group_count = var.allowed_inbound_security_group_count

  serf_lan_port = var.serf_lan_port
}

