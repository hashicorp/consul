# Terraform Modules

This folder contains modules for Terraform that can setup Consul for
various systems. The infrastructure provider that is used is designated
by the folder above. See the `variables.tf` file in each for more documentation. 

To deploy Consul in multiple Subnets/AZ on AWS - supply:  -var 'vpc_id=vpc-1234567' -var 'subnets={ "0" = "subnet-12345", "1" = "subnet-23456", "2" = "subnet-34567"}'
