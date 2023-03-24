variable "vpc_name" {
  description = "Name of the VPC"
}

variable "ami_owners" {
  type        = list(string)
  description = "The account owner number which the desired AMI is in"
}

variable "role_arn" {
  type        = string
  description = "Role ARN for assume role"
}

variable "consul_download_url" {
  type        = string
  description = "URL to download the Consul binary from"
  default     = ""
}
variable "cluster_name" {
  description = "What to name the Consul cluster and all of its associated resources"
  type        = string
  default     = "consul-example"
}

variable "cluster_tag_key" {
  description = "The tag the EC2 Instances will look for to automatically discover each other and form a cluster."
  type        = string
  default     = "consul-ci-load-test"
}
