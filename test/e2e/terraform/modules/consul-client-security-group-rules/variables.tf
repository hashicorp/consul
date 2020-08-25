# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "security_group_id" {
  description = "The ID of the security group to which we should add the Consul security group rules"
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
  default     = 0
}

variable "serf_lan_port" {
  description = "The port used to handle gossip in the LAN. Required by all agents."
  default     = 8301
}

