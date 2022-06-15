# Terraform Consul Load Testing
## How to use
1. Build an image with the desired Consul version and a load test image in the Packer folder [here](../packer).
2. Create your own `vars.tfvars` file in this directory.
3. Place the appropriate AMI IDs in the `consul_ami_id` and `test_server_ami` variables. If no AMI ID is specified it will default
to pulling from latest.
4. Set either `consul_version` or `consul_download_url`. If neither is set it will default to utilizing Consul 1.9.0
5. AWS Variables are set off of environment variables. Make sure to export necessary variables [shown here](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#environment-variables).
6. Run `terraform init` once to setup the working directory.
7. Run `terraform plan -var-file=vars.tfvars`, and then `terraform apply -var-file=vars.tfvars` when ready.
8. Upon completion k6 should run and push metrics to the desired Datadog dashboard.

An example of a `vars.tfvars` :

```
vpc_name             = "consul-test-vpc"
vpc_cidr             = "11.0.0.0/16"
vpc_allwed_ssh_cidr  = "0.0.0.0/0"
public_subnet_cidrs  = ["11.0.1.0/24", "11.0.3.0/24"]
private_subnet_cidrs = ["11.0.2.0/24"]
vpc_az               = ["us-east-2a", "us-east-2b"]
test_instance_type   = "t2.micro"
test_server_ami      = "ami-0ad7711e837ebe166"
cluster_name         = "ctest"
test_public_ip       = "true"
instance_type        = "t2.micro"
ami_owners           = ["******"]
consul_ami_id        = "ami-016d80ff5472346f0"
````

Note that `vpc_allwed_ssh_cidr` must be set to allowed the test server to be accessible from the
machine running the load test, e.g., "0.0.0.0/0" (It is disabled by default).

## Customization
All customization for infrastructure that is available can be found by looking through the `variables.tf` file.
 
## How to SSH
After `terraform apply` is run Terraform should create a `keys/` directory which will give access to all instances created.
For example, `ssh -i "keys/[cluster-name]-spicy-banana.pem" ubuntu@[IPADDRESS]`

