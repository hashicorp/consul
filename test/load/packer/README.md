## Quick start

# Consul AMI:

Within the `consul-ami/` directory

1) Update the `variables` section of the `consul.json` Packer template to configure the AWS region and datadog api key you would like to use. Feel free to reference this article to find your [datadog API key](https://docs.datadoghq.com/account_management/api-app-keys/#api-keys). 
2) Run `packer build consul.json`. 

When the build finishes, it will output the IDs of the new AMI. Add this AMI ID in the `consul_ami_id` variable in the `vars.tfvars` file.

For additional customization you can add [tags](https://docs.datadoghq.com/getting_started/tagging/assigning_tags/?tab=noncontainerizedenvironments) within the `scripts/datadog.yaml` file. One example of a tag could be `"consul_version" : "consulent_175"`. These tags are searchable through the datadog dashboard. Another form of customization is changing the datacenter tag within `scripts/telemetry.json`, however it is defaulted to `us-east-1`.


# Load Test AMI

Within the `loadtest-ami/` directory

1) Set the AWS region in the `loadtest.json` file
2) Run the command `packer build loadtest.json` 

The script that k6 runs is found within `scripts/loadtest.js`. This script can be updated to send request to more Consul endpoints. For additional information on k6 please check out their [guides](https://k6.io/docs/getting-started/running-k6). 
