# Consul AMI

## Quick start

To build the Consul AMI:

1. `git clone` this repo to your computer.
2. Install [Packer](https://www.packer.io/).
3. Configure your AWS credentials using one of the [options supported by the AWS
   SDK](http://docs.aws.amazon.com/sdk-for-java/v1/developer-guide/credentials.html). Usually, the easiest option is to
   set the `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_DEFAULT_REGION` environment variables.
4. Update the `variables` section of the `consul.json` Packer template to configure the AWS region and datadog api key you would like to use. Feel free to reference this article to find your [datadog API key](https://docs.datadoghq.com/account_management/api-app-keys/#api-keys). 
5. For additional customization you can add [tags](https://docs.datadoghq.com/getting_started/tagging/assigning_tags/?tab=noncontainerizedenvironments) within the `scripts/datadog.yaml` file. One example of a tag could be `"consul_version" : "consulent_175"`. These tags are searchable through the datadog dashboard. Another form of customization is changing the datacenter tag within `scripts/telemetry.json`, however it is defaulted to `us-east-1`.
6. Run `packer build consul.json`. 

When the build finishes, it will output the IDs of the new AMI. Add this AMI ID in the `consul_ami_id` variable in the `vars.tfvars` file.
