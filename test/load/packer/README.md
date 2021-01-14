## Quick start

# Consul AMI:

Within the `consul-ami/` directory
1) Retrieve your [Datadog API key]((https://docs.datadoghq.com/account_management/api-app-keys/#api-keys)), set this as an environment variable, ex: `export DD_API_KEY=$YOURDDAPIKEYHERE`
2) Set the AWS_DEFAULT_REGION for Packer, ex: `export AWS_DEFAULT_REGION=us-east-1`
3) Run `packer build consul.json`. 

When the build finishes, it will output the IDs of the new AMI. Save those AMI IDs as they will be used later as variables needed for Terraform! 

For additional customization you can add [tags](https://docs.datadoghq.com/getting_started/tagging/assigning_tags/?tab=noncontainerizedenvironments) within the `scripts/datadog.yaml` file. An example of a tag could be `"consul_version" : "consulent_175"`. These tags are searchable through the datadog dashboard. Another form of customization is changing the datacenter tag within `scripts/telemetry.json`, however it is defaulted to `us-east-1`.


# Load Test AMI

Within the `loadtest-ami/` directory

1) Set the AWS_DEFAULT_REGION for Packer, ex: `export AWS_DEFAULT_REGION=us-east-1`
2) Run the command `packer build loadtest.json` 

The script that k6 runs is found within `scripts/loadtest.js`. This script can be updated to send requests to more Consul endpoints. For additional information on k6 please check out their [guides](https://k6.io/docs/getting-started/running-k6). 
