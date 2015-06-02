## Running the aws templates to set up a consul cluster

The platform variable defines the target OS, default is ubuntu, rhel6 is an option

For AWS provider, set up your AWS environment as outlined in https://www.terraform.io/docs/providers/aws/index.html

To set up ubuntu based, run like below, replace key_name and key_path with actual values

terraform apply -var 'key_name=consul' -var 'key_path=/Users/xyz/consul.pem'

or 

terraform apply -var 'key_name=consul' -var 'key_path=/Users/xyz/consul.pem' -var 'platform=ubuntu'

To run rhel6, run like below

terraform apply -var 'key_name=consul' -var 'key_path=/Users/xyz/consul.pem' -var 'platform=rhel6'