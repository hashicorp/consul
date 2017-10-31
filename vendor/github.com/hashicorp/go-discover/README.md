# Go Discover Nodes for Cloud Providers [![Build Status](https://travis-ci.org/hashicorp/go-discover.svg?branch=master)](https://travis-ci.org/hashicorp/go-discover) [![GoDoc](https://godoc.org/github.com/hashicorp/go-discover?status.svg)](https://godoc.org/github.com/hashicorp/go-discover)


`go-discover` is a Go (golang) library and command line tool to discover
ip addresses of nodes in cloud environments based on meta information
like tags provided by the environment.

The configuration for the providers is provided as a list of `key=val key=val
...` tuples. If either the key or the value contains a space (` `), a backslash
(`\`) or double quotes (`"`) then it needs to be quoted with double quotes.
Within a quoted string you can use the backslash to escape double quotes or the
backslash itself, e.g. `key=val "some key"="some value"`

Duplicate keys are reported as error and the provider is determined through the
`provider` key. 

### Supported Providers

The following cloud providers have implementations in the go-discover/provider
sub packages. Additional providers can be added through the
[Register](https://godoc.org/github.com/hashicorp/go-discover#Register)
function.

 * Aliyun (Alibaba) Cloud [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/aliyun/aliyun_discover.go#L15-L28)
 * Amazon AWS [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/aws/aws_discover.go#L19-L33)
 * DigitalOcean [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/digitalocean/digitalocean_discover.go#L16-L24)
 * Google Cloud [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/gce/gce_discover.go#L17-L37)
 * Microsoft Azure [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/azure/azure_discover.go#L16-L30)
 * Openstack [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/os/os_discover.go#L23-L38)
 * Scaleway [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/scaleway/scaleway_discover.go#L14-L22)
 * SoftLayer [Config options](https://github.com/hashicorp/go-discover/blob/master/provider/softlayer/softlayer_discover.go#L16-L25)

### Config Example

```
# Aliyun (Alibaba) Cloud
provider=aliyun region=... tag_key=consul tag_value=... access_key_id=... access_key_secret=...

# Amazon AWS
provider=aws region=eu-west-1 tag_key=consul tag_value=... access_key_id=... secret_access_key=...

# DigitalOcean
provider=digitalocean region=... tag_name=... api_token=...

# Google Cloud
provider=gce project_name=... zone_pattern=eu-west-* tag_value=consul credentials_file=...

# Microsoft Azure
provider=azure tag_name=consul tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=...

# Openstack
provider=os tag_key=consul tag_value=server username=... password=... auth_url=...

# Scaleway
provider=scaleway organization=my-org tag_name=consul-server token=... region=...

# SoftLayer
provider=softlayer datacenter=dal06 tag_value=consul username=... api_key=...

```

## Command Line Tool Usage

Install the command line tool with:

```
go get -u github.com/hashicorp/go-discover/cmd/discover
```

Then run it with:

```
$ discover addrs provider=aws region=eu-west-1 ...
```

## Library Usage

Install the library with:

```
go get -u github.com/hashicorp/go-discover
```

You can then either support discovery for all available providers
or only for some of them.

```go
// support discovery for all supported providers
d := discover.Discover{}

// support discovery for AWS and GCE only
d := discover.Discover{
	Providers : map[string]discover.Provider{
		"aws": discover.Providers["aws"],
		"gce": discover.Providers["gce"],
	}
}

// use ioutil.Discard for no log output
l := log.New(os.Stderr, "", log.LstdFlags)

cfg := "provider=aws region=eu-west-1 ..."
addrs, err := d.Addrs(cfg, l)
```

For complete API documentation, see
[GoDoc](https://godoc.org/github.com/hashicorp/go-discover). The configuration
for the supported providers is documented in the
[providers](https://godoc.org/github.com/hashicorp/go-discover/provider)
sub-package.
