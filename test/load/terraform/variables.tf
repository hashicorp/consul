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

variable "ssh_key_name" {
  description = "The name of an EC2 Key Pair that can be used to SSH to the EC2 Instances in this cluster. Set to an empty string to not associate a Key Pair."
  type        = string
  default     = null
}

variable "vpc_id" {
  description = "The ID of the VPC in which the nodes will be deployed.  Uses default VPC if not supplied."
  type        = string
  default     = null
}

variable "spot_price" {
  description = "The maximum hourly price to pay for EC2 Spot Instances."
  type        = number
  default     = null
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
}

variable "test_instance_type" {
  type        = string
  description = "AWS Instance type for all test servers"
}

variable "test_public_ip" {
  type        = bool
  description = "Should the test servers have a public IP?"
}

variable "instance_type" {
  type        = string
  description = "Instance Type for all instances in the Consul Cluster"
}

variable "ami_owners" {
  type        = list(string)
  description = "The account owner number which the desired AMI is in"
}
