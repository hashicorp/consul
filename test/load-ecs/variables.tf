# ---------------------------------------------------------------------------------------------------------------------
# ENVIRONMENT VARIABLES
# Define these secrets as environment variables
# ---------------------------------------------------------------------------------------------------------------------

# AWS_ACCESS_KEY_ID
# AWS_SECRET_ACCESS_KEY
# AWS_DEFAULT_REGION

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "cluster_name" {
  description = "What to name the Consul cluster and all of its associated resources"
  type        = string
  default     = "consul-example"
}

variable "aws_region" {
  description = "What region the cluster and resources will be deployed into."
  type        = string
  default     = "us-east-1"
}

variable "vpc_az" {
  type        = list(string)
  description = "VPC Availability Zone"
  validation {
    condition     = length(var.vpc_az) >= 2
    error_message = "VPC needs at least two Availability Zones for ALB to work."
  }
}

variable "vpc_name" {
  description = "Name of the VPC"
}

variable "vpc_cidr" {
  description = "List of CIDR blocks for the VPC module"
}

variable "vpc_allowed_ssh_cidr" {
  description = "List of CIDR blocks allowed to ssh to the test server; set to 0.0.0.0/0 to allow access from anywhere"
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidrs" {
  type        = list(string)
  description = "CIDR Block for the Public Subnet, must be within VPC CIDR range"
}

variable "private_subnet_cidrs" {
  type        = list(string)
  description = "CIDR Block for the Private Subnet, must be within VPC CIDR range"
}

// new variables
variable "deploy_consul_ecs" {
  type    = bool
  default = false
}

variable "consul_image_name" {
  type        = string
  description = "Name of the Consul Agent Docker Image"
  default     = "consul"
}

variable "consul_image_tag" {
  type        = string
  description = "Tag of the Consul Agent Docker Image"
  default     = "local"
}

variable "internal_alb_listener" {
  type        = bool
  description = "ALB is internal-only to reduce load-test costs. When false the ALB will be accessible over the public network."
  default     = true
}

variable "deploy_efs_cluster" {
  type    = bool
  default = false
}

variable "run_k6" {
  type    = bool
  default = false
}

variable "datadog_apikey" {
  description = "Datadog API key"
  type        = string
  default     = ""
}

variable "k6_apikey" {
  description = "K6 API key"
  type        = string
  default     = ""
}

variable "consul_encryption_token" {
  description = "Consul Gossip Encryption Token"
  type        = string
}

variable "admin_cidrs" {
  default = []
}

variable "enable_container_init" {
  default = false
}
