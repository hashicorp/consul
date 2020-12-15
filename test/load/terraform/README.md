## Terraform Consul Load Testing

# How to use
1. Build an image with the desired Consul version and a loadtest image in the Packer folder [here](../packer).
2. Create your own `vars.tfvars` file in this directory.
3. Place the appropriate AMI IDs in the `consul_ami_id` and `test_server_ami` variables, here is an example of a `vars.tfvars`:
```
vpc_name             = "consul-test-vpc"
vpc_cidr             = "11.0.0.0/16"
public_subnet_cidrs  = ["11.0.1.0/24", "11.0.3.0/24"]
private_subnet_cidrs = ["11.0.2.0/24"]
vpc_az               = ["us-east-2a", "us-east-2b"]
test_instance_type   = "t2.micro"

## This is found from building the image in packer/loadtest-ami
test_server_ami      = "ami-0ad7711e837ebe166"
cluster_name         = "ctest"
test_public_ip       = "true"
instance_type        = "t2.micro"
ami_owners           = ["******"]

## This is found from building the image in packer/consul-ami
consul_ami_id        = "ami-016d80ff5472346f0"
```

If `consul_version` or `consul_download_url` is not set within the Terraform variables it will default to utilziing Consul 1.9.0 

4. AWS Variables are set off of environment variables. Make sure to export nessecary variables [shown here](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#environment-variables).
5. Run `terraform plan -var-file=vars.tfvars`, and then `terraform apply -var-file=vars.tfvars` when ready.
6. Upon completion k6 should run and push metrics to desired Datadog dashboard. 

# Customization 
All customization for infrastructure that is available can be found by looking through the `variables.tf` file.

# How to SSH
After `terraform apply` is ran Terraform should create a `keys/` directory which will give access to all instances created. 
