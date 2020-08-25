# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "security_group_id" {
  description = "The ID of the security group to which we should add the Consul security group rules"
  type        = string
}

variable "allowed_inbound_cidr_blocks" {
  description = "A list of CIDR-formatted IP address ranges from which the EC2 Instances will allow connections to Consul"
  type        = list(string)
  default     = []
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "allowed_inbound_security_group_ids" {
  description = "A list of security group IDs that will be allowed to connect to Consul"
  type        = list(string)
  default     = []
}

variable "allowed_inbound_security_group_count" {
  description = "The number of entries in var.allowed_inbound_security_group_ids. Ideally, this value could be computed dynamically, but we pass this variable to a Terraform resource's 'count' property and Terraform requires that 'count' be computed with literals or data sources only."
  type        = number
  default     = 0
}

variable "server_rpc_port" {
  description = "The port used by servers to handle incoming requests from other agents."
  type        = number
  default     = 8300
}

variable "cli_rpc_port" {
  description = "The port used by all agents to handle RPC from the CLI."
  type        = number
  default     = 8400
}

variable "serf_lan_port" {
  description = "The port used to handle gossip in the LAN. Required by all agents."
  type        = number
  default     = 8301
}

variable "serf_wan_port" {
  description = "The port used by servers to gossip over the WAN to other servers."
  type        = number
  default     = 8302
}

variable "http_api_port" {
  description = "The port used by clients to talk to the HTTP API"
  type        = number
  default     = 8500
}

variable "dns_port" {
  description = "The port used to resolve DNS queries."
  type        = number
  default     = 8600
}

