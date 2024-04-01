# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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

variable "consul_ami_id" {
  description = "The ID of the AMI to run in the cluster. This should be an AMI built from the Packer template under examples/consul-ami/consul.json. To keep this example simple, we run the same AMI on both server and client nodes, but in real-world usage, your client nodes would also run your apps. If the default value is used, Terraform will look up the latest AMI build automatically."
  type        = string
  default     = null
}

variable "cluster_name" {
  description = "What to name the Consul cluster and all of its associated resources"
  type        = string
  default     = "consul-example"
}

variable "num_servers" {
  description = "The number of Consul server nodes to deploy. We strongly recommend using 3 or 5."
  type        = number
  default     = 3
}

variable "num_clients" {
  description = "The number of Consul client nodes to deploy. You typically run the Consul client alongside your apps, so set this value to however many Instances make sense for your app code."
  type        = number
  default     = 2
}

variable "cluster_tag_key" {
  description = "The tag the EC2 Instances will look for to automatically discover each other and form a cluster."
  type        = string
  default     = "consul-servers"
}

variable "vpc_az" {
  type        = list(string)
  description = "VPC Availability Zone"
  validation {
    condition     = length(var.vpc_az) == 2
    error_message = "VPC needs at least two Availability Zones for ALB to work."
  }
}

variable "vpc_name" {
  description = "Name of the VPC"
}

variable "vpc_cidr" {
  description = "List of CIDR blocks for the VPC module"
}

variable "vpc_allwed_ssh_cidr" {
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

variable "test_server_ami" {
  type        = string
  description = "The AMI ID from the Packer generated image"
  default     = null
}

variable "test_instance_type" {
  type        = string
  description = "AWS Instance type for all test servers"
  default     = "t2.small"
}

variable "test_public_ip" {
  type        = bool
  description = "Should the test servers have a public IP?"
}

variable "instance_type" {
  type        = string
  description = "Instance Type for all instances in the Consul Cluster"
  default     = "m5n.large"
}

variable "ami_owners" {
  type        = list(string)
  description = "The account owner number which the desired AMI is in"
}

variable "consul_download_url" {
  type        = string
  description = "URL to download the Consul binary from"
  default     = ""
}

variable "consul_version" {
  type        = string
  description = "Version of the Consul binary to install"
  default     = "1.12.0"
}
