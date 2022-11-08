terraform {
  required_version = ">= 0.13"
}
# ---------------------------------------------------------------------------------------------------------------------
# Create variables and ssh keys
# ---------------------------------------------------------------------------------------------------------------------

resource "random_pet" "test" {
}

locals {
  random_name = "${var.cluster_name}-${random_pet.test.id}"
}

# ---------------------------------------------------------------------------------------------------------------------
# Create VPC with public and also private subnets
# ---------------------------------------------------------------------------------------------------------------------

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "2.21.0"

  name               = "${local.random_name}-${var.vpc_name}"
  cidr               = var.vpc_cidr
  azs                = var.vpc_az
  public_subnets     = var.public_subnet_cidrs
  private_subnets    = var.private_subnet_cidrs
  enable_nat_gateway = true

  # Specifically for EFS mount via dns feature
  enable_dns_hostnames = true
  enable_dns_support   = true
}
