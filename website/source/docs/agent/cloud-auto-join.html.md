---
layout: "docs"
page_title: "Cloud Auto-join"
sidebar_current: "docs-agent-cloud-auto-join"
description: |-
  Consul supports automatic cluster joining using cloud metadata on various providers.
---

# Cloud Auto-joining

As of Consul 0.9.1, `retry-join` accepts a unified interface using the
[go-discover](https://github.com/hashicorp/go-discover) library for doing
automatic cluster joining using cloud metadata. To use retry-join with a
supported cloud provider, specify the configuration on the command line or
configuration file as a `key=value key=value ...` string.

In Consul 0.9.1-0.9.3 the values need to be URL encoded but for most
practical purposes you need to replace spaces with `+` signs.

As of Consul 1.0 the values are taken literally and must not be URL
encoded. If the values contain spaces, backslashes or double quotes then
they need to be double quoted and the usual escaping rules apply.

```sh
$ consul agent -retry-join 'provider=my-cloud config=val config2="some other val" ...'
```

or via a configuration file:

```json
{
    "retry_join": ["provider=my-cloud config=val config2=\"some other val\" ..."]
}
```

The cloud provider-specific configurations are detailed below. This can be
combined with static IP or DNS addresses or even multiple configurations
for different providers.

In order to use discovery behind a proxy, you will need to set
`HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables per
[Golang `net/http` library](https://golang.org/pkg/net/http/#ProxyFromEnvironment).

The following sections give the options specific to each supported cloud
provider.

### Amazon EC2

This returns the first private IP address of all servers in the given
region which have the given `tag_key` and `tag_value`.

```sh
$ consul agent -retry-join "provider=aws tag_key=... tag_value=..."
```

```json
{
    "retry_join": ["provider=aws tag_key=... tag_value=..."]
}
```

- `provider` (required) - the name of the provider ("aws" in this case).
- `tag_key` (required) - the key of the tag to auto-join on.
- `tag_value` (required) - the value of the tag to auto-join on.
- `region` (optional) - the AWS region to authenticate in.
- `addr_type` (optional) - the type of address to discover: `private_v4`, `public_v4`, `public_v6`. Default is `private_v4`. (>= 1.0)
- `access_key_id` (optional) - the AWS access key for authentication (see below for more information about authenticating).
- `secret_access_key` (optional) - the AWS secret access key for authentication (see below for more information about authenticating).

#### Authentication &amp; Precedence

- Static credentials `access_key_id=... secret_access_key=...`
- Environment variables (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`)
- Shared credentials file (`~/.aws/credentials` or the path specified by `AWS_SHARED_CREDENTIALS_FILE`)
- ECS task role metadata (container-specific).
- EC2 instance role metadata.

The only required IAM permission is `ec2:DescribeInstances`, and it is
recommended that you make a dedicated key used only for auto-joining. If the
region is omitted it will be discovered through the local instance's [EC2
metadata
endpoint](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html).

### Microsoft Azure

This returns the first private IP address of all servers in the given region
which have the given `tag_key` and `tag_value` in the tenant and subscription, or in
the given `resource_group` of a `vm_scale_set` for Virtual Machine Scale Sets.

```sh
$ consul agent -retry-join "provider=azure tag_name=... tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."
```

```json
{
    "retry_join": ["provider=azure tag_name=... tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."]
}
```

- `provider` (required) - the name of the provider ("azure" in this case).
- `tenant_id` (required) - the tenant to join machines in.
- `client_id` (required) - the client to authenticate with.
- `secret_access_key` (required) - the secret client key.

Use these configuration parameters when using tags:
- `tag_name` - the name of the tag to auto-join on.
- `tag_value` - the value of the tag to auto-join on.

Use these configuration parameters when using Virtual Machine Scale Sets (Consul 1.0.3 and later):
- `resource_group` - the name of the resource group to filter on.
- `vm_scale_set` - the name of the virtual machine scale set to filter on.

When using tags the only permission needed is the `ListAll` method for `NetworkInterfaces`. When using
Virtual Machine Scale Sets the only role action needed is `Microsoft.Compute/virtualMachineScaleSets/*/read`.

### Google Compute Engine

This returns the first private IP address of all servers in the given
project which have the given `tag_value`.

```sh
$ consul agent -retry-join "provider=gce project_name=... tag_value=..."
```

```json
{
    "retry_join": ["provider=gce project_name=... tag_value=..."]
}
```

- `provider` (required) - the name of the provider ("gce" in this case).
- `tag_value` (required) - the value of the tag to auto-join on.
- `project_name` (optional) - the name of the project to auto-join on. Discovered if not set.
- `zone_pattern` (optional) - the list of zones can be restricted through an RE2 compatible regular expression. If omitted, servers in all zones are returned.
- `credentials_file` (optional) - the credentials file for authentication. See below for more information.

#### Authentication &amp; Precedence

- Use credentials from `credentials_file`, if provided.
- Use JSON file from `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
- Use JSON file in a location known to the gcloud command-line tool.
    - On Windows, this is `%APPDATA%/gcloud/application_default_credentials.json`.
    - On other systems, `$HOME/.config/gcloud/application_default_credentials.json`.
- On Google Compute Engine, use credentials from the metadata
    server. In this final case any provided scopes are ignored.

Discovery requires a [GCE Service
Account](https://cloud.google.com/compute/docs/access/service-accounts).
Credentials are searched using the following paths, in order of precedence.

### IBM SoftLayer

This returns the first private IP address of all servers for the given
datacenter with the given `tag_value`.

```sh
$ consul agent -retry-join "provider=softlayer datacenter=... tag_value=... username=... api_key=..."
```

```json
{
    "retry_join": ["provider=softlayer datacenter=... tag_value=... username=... api_key=..."]
}
```

- `provider` (required) - the name of the provider ("softlayer" in this case).
- <a name="sl_datacenter"></a><a href="#sl_datacenter"><code>datacenter</code></a></a> (required) - the name of the datacenter to auto-join in.
- `tag_value` (required) - the value of the tag to auto-join on.
- `username` (required) - the username to use for auth.
- `api_key` (required) - the api key to use for auth.

### Aliyun (Alibaba Cloud)

This returns the first private IP address of all servers for the given
`region` with the given `tag_key` and `tag_value`.

```sh
$ consul agent -retry-join "provider=aliyun region=... tag_key=consul tag_value=... access_key_id=... access_key_secret=..."
```

```json
{
    "retry_join": ["provider=aliyun region=... tag_key=consul tag_value=... access_key_id=... access_key_secret=..."]
}
```

- `provider` (required) - the name of the provider ("aliyun" in this case).
- `region` (required) - the name of the region.
- `tag_key` (required) - the key of the tag to auto-join on.
- `tag_value` (required) - the value of the tag to auto-join on.
- `access_key_id` (required) -the access key to use for auth.
- `access_key_secret` (required) - the secret key to use for auth.

The required RAM permission is `ecs:DescribeInstances`.
It is recommended you make a dedicated key used only for auto-joining.

### Digital Ocean

This returns the first private IP address of all servers for the given
`region` with the given `tag_name`.

```sh
$ consul agent -retry-join "provider=digitalocean region=... tag_name=... api_token=..."
```

```json
{
    "retry_join": ["provider=digitalocean region=... tag_name=... api_token=..."]
}
```

- `provider` (required) - the name of the provider ("digitalocean" in this case).
- `region` (required) - the name of the region.
- `tag_name` (required) - the value of the tag to auto-join on.
- `api_token` (required) -the token to use for auth.

### Openstack

This returns the first private IP address of all servers for the given
`region` with the given `tag_key` and `tag_value`.

```sh
$ consul agent -retry-join "provider=os tag_key=consul tag_value=server username=... password=... auth_url=..."
```

```json
{
    "retry_join": ["provider=os tag_key=consul tag_value=server username=... password=... auth_url=..."]
}
```

- `provider` (required) - the name of the provider ("os" in this case).
- `tag_key` (required) - the key of the tag to auto-join on.
- `tag_value` (required) - the value of the tag to auto-join on.
- `project_id` (optional) - the id of the project (tenant id).
- `username` (optional) - the username to use for auth.
- `password` (optional) - the password to use for auth.
- `token` (optional) - the token to use for auth.
- `auth_url` (optional) - the identity endpoint to use for auth.
- `insecure` (optional) - indicates whether the API certificate should not be checked. Any value means `true`.

The configuration can also be provided by environment variables.

### Scaleway

This returns the first private IP address of all servers for the given
`region` with the given `tag_name`.

```sh
$ consul agent -retry-join "provider=scaleway organization=my-org tag_name=consul-server token=... region=..."
```

```json
{
    "retry_join": ["provider=scaleway organization=my-org tag_name=consul-server token=... region=..."]
}
```

- `provider` (required) - the name of the provider ("scaleway" in this case).
- `region` (required) - the name of the region.
- `tag_name` (required) - the name of the tag to auto-join on.
- `organization` (required) - the organization access key to use for auth (equal to access key).
- `token` (required) - the token to use for auth.

### Joyent Triton

This returns the first PrimaryIP addresses for all servers with the given 
`tag_key` and `tag_value`.

```sh
$ consul agent -retry-join "provider=triton account=testaccount url=https://us-sw-1.api.joyentcloud.com key_id=... tag_key=consul-role tag_value=server"
```

```json
{
    "retry_join": ["provider=triton account=testaccount url=https://us-sw-1.api.joyentcloud.com key_id=... tag_key=consul-role tag_value=server"]
}
```

- `provider` (required) - the name of the provider ("triton" in this case).
- `account` (required) - the name of the account.
- `url` (required) - the URL of the Triton api endpoint to use.
- `key_id` (required) - the key id to use.
- `tag_key` (optional) - the instance tag key to use.
- `tag_value` (optional) - the tag value to use.
